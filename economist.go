package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"time"
)

const (
	ScalingFactor = "3"   // volume scaling factor
	TempoFactor   = "1.7" // tempo (speed up) factor
)

var Concurrent = runtime.NumCPU()
var Zipfile = filepath.Join(os.Getenv("HOME"), "Downloads", "*The*Economist*.zip")

var (
	IsSourceFile        = regexp.MustCompile(`^(?:Issue *\d+ *- *)?(\d+) (.*?) - (.*\.mp3)$`)
	NonWord             = regexp.MustCompile(`[^\w\.]+`)
	Underscores         = regexp.MustCompile(`__+`)
	LeadTrailUnderscore = regexp.MustCompile(`^_|_$`)
	Apostrophe          = regexp.MustCompile(`'`)
	LonelyS1            = regexp.MustCompile(`_s_`)
	LonelyS2            = regexp.MustCompile(`_s `)
	LonelyS3            = regexp.MustCompile(`_s\.`)
	TargetName          = regexp.MustCompile(`/\d+-([^/]+)/\d+-([^/]+)\.mp3$`)
)

type Pair struct {
	Source     string
	Target     string
	TargetFile *os.File
}

func main() {
	log.SetFlags(log.Ltime)
	start := time.Now()

	// make sure we have the latest edition downloaded
	var zipfile string
	if len(os.Args) < 2 {
		ziplist, err := filepath.Glob(Zipfile)
		if err != nil {
			log.Fatal("finding zip file: ", err)
		}
		if len(ziplist) == 0 {
			log.Fatal("no zip file found")
		}
		sort.Strings(ziplist)
		zipfile = ziplist[len(ziplist)-1]
	} else if len(os.Args) == 2 {
		zipfile = os.Args[1]
	} else {
		log.Fatalf("Usage: %s [filename]", os.Args[0])
	}
	_, filename := filepath.Split(zipfile)
	log.Print(filename)

	// unzip this week
	log.Print("Unzipping this week's issue...")
	contents := make(map[string][]byte)
	files := []string{}
	r, err := zip.OpenReader(zipfile)
	if err != nil {
		log.Fatalf("opening %s: %v", zipfile, err)
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			log.Fatalf("opening file %s: %v", f.Name, err)
		}
		data, err := ioutil.ReadAll(rc)
		if err != nil {
			log.Fatalf("reading file %s: %v", f.Name, err)
		}
		rc.Close()
		contents[f.Name] = data
		files = append(files, f.Name)
	}
	r.Close()

	// blow away last week on the SD drive
	log.Print("Clearing last week from SD drive...")
	if err = os.RemoveAll(Target); err != nil {
		log.Fatal("clearing SD drive: ", err)
	}
	if err = os.Mkdir(Target, 0755); err != nil {
		log.Fatalf("mkdir %s: %v", Target, err)
	}
	//sync()

	// kill section intros and rearrange into a directory per section
	var script []*Pair
	var oldsec, secfolder string
	seccount := -1
	for _, elt := range files {
		pieces := IsSourceFile.FindStringSubmatch(elt)
		if len(pieces) != 4 {
			continue
		}
		track, section, article := pieces[1], pieces[2], pieces[3]

		section = NonWord.ReplaceAllString(section, "_")
		section = Underscores.ReplaceAllString(section, "_")
		section = LeadTrailUnderscore.ReplaceAllString(section, "")

		article = Apostrophe.ReplaceAllString(article, "")
		article = NonWord.ReplaceAllString(article, "_")
		article = Underscores.ReplaceAllString(article, "_")
		article = LonelyS1.ReplaceAllString(article, "s_")
		article = LonelyS2.ReplaceAllString(article, "s ")
		article = LonelyS3.ReplaceAllString(article, "s.")
		article = LeadTrailUnderscore.ReplaceAllString(article, "")

		if SkipSections[section] {
			if section != oldsec {
				oldsec = section
				log.Printf("Skipping section %s", section)
			}
			continue
		}

		if section != oldsec {
			oldsec = section

			seccount++
			secfolder = filepath.Join(Target, fmt.Sprintf("%02d-%s", seccount, section))
			if err = os.Mkdir(secfolder, 0755); err != nil {
				log.Fatalf("mkdir %s: %v", secfolder, err)
			}
		}
		targetName := filepath.Join(secfolder, fmt.Sprintf("%s-%s", track, article))
		targetFile, err := os.Create(targetName)
		if err != nil {
			log.Fatalf("creating out file %s: %v", targetName, err)
		}
		script = append(script, &Pair{elt, targetName, targetFile})
	}

	// now actually do the copying/encoding
	available := make(chan struct{}, Concurrent)
	ack := make(chan struct{})

	// handle each individual job
	go func() {
		section := ""
		for i, job := range script {
			// get a slot
			available <- struct{}{}

			pieces := TargetName.FindStringSubmatch(job.Target)
			if len(pieces) != 3 {
				panic("Bad file name in script: " + job.Target)
			}
			newsection, article := pieces[1], pieces[2]
			if newsection != section {
				section = newsection
				log.Print("Section: ", section)
			}

			go func(pair *Pair, article string, n int) {
				log.Print("    ", article)

				// write the file to a temporary location
				temp1 := filepath.Join(os.TempDir(), fmt.Sprintf("economist-%d.mp3", n))
				if err := ioutil.WriteFile(temp1, contents[pair.Source], 0644); err != nil {
					log.Fatalf("Error writing file %s: %v", temp1, err)
				}
				defer os.Remove(temp1)

				// copy the file over and change the tempo
				temp2 := filepath.Join(os.TempDir(), fmt.Sprintf("economist-%d-sox.mp3", n))
				defer os.Remove(temp2)
				cmd := exec.Command(
					"sox",
					temp1,
					temp2,
					"tempo", "-s", TempoFactor)
				if err = cmd.Run(); err != nil {
					log.Fatal("running sox job for for ", pair.Target, ": ", err)
				}

				// copy the adjusted file to its final location
				src, err := os.Open(temp2)
				if err != nil {
					log.Fatal("opening ", temp2, " for copying: ", err)
				}
				defer src.Close()

				if _, err = io.Copy(pair.TargetFile, src); err != nil {
					log.Fatal("copying ", temp2, " to ", pair.Target, ": ", err)
				}
				pair.TargetFile.Close()

				// sync
				//fp, err := os.Open(pair.Target)
				//if err != nil {
				//	log.Fatalf("open %s: %v", pair.Target, err)
				//}
				//defer fp.Close()
				//if err = fp.Sync(); err != nil {
				//	log.Fatal("syncing %s: %v", pair.Target, err)
				//}

				ack <- struct{}{}
				<-available
			}(job, article, i)

			// pause a bit so it can (probably) get started
			// before we launch the next one
			// this helps keep the files in the file system mostly
			// in play order
			time.Sleep(100 * time.Millisecond)
		}

	}()

	// wait until all jobs are finished
	for _, _ = range script {
		<-ack
	}

	// final sync
	sync()
	sync()

	elapsed := time.Since(start)
	log.Printf("Finished in %v", elapsed-elapsed%time.Second)
}
