package main

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	//"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"syscall"
	"time"
)

const (
	ScalingFactor = "3" // volume scaling factor
)

var Concurrent = runtime.NumCPU()
var Zipfile = os.Getenv("HOME") + "/Downloads/*The*Economist*.zip"
var Target = "/media/" + os.Getenv("USER") + "/economist/ec"

var SkipSections = map[string]bool{
	"The_Americas":           true,
	"Asia":                   true,
	"China":                  true,
	"Middle_East_and_Africa": true,
	"Europe":                 true,
}

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
	Source string
	Target string
}

func main() {
	start := time.Now()

	// make sure we have the latest edition downloaded
	var zipfile string
	if len(os.Args) < 2 {
		ziplist, err := filepath.Glob(Zipfile)
		if err != nil {
			log.Fatal("Finding zip file: ", err)
		}
		if len(ziplist) == 0 {
			log.Fatal("No zip file found")
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
		log.Fatalf("Opening %s: %v", zipfile, err)
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			log.Fatalf("Error opening file %s: %v", f.Name, err)
		}
		data, err := ioutil.ReadAll(rc)
		if err != nil {
			log.Fatalf("Error reading file %s: %v", f.Name, err)
		}
		rc.Close()
		contents[f.Name] = data
		files = append(files, f.Name)
	}
	r.Close()

	// blow away last week on the ЅD drive
	log.Print("Clearing last week from SD drive...")
	if err = os.RemoveAll(Target); err != nil {
		log.Fatal("Clearing SD drive: ", err)
	}
	if err = os.Mkdir(Target, 0755); err != nil {
		log.Fatal("Making target directory: ", err)
	}
	syscall.Sync()

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
			secfolder = fmt.Sprintf("%s/%02d-%s", Target, seccount, section)
			if err = os.Mkdir(secfolder, 0755); err != nil {
				log.Fatal("Creating section folder ", secfolder, ": ", err)
			}
		}
		script = append(script, &Pair{
			elt,
			fmt.Sprintf("%s/%s-%s", secfolder, track, article),
		})
	}

	// now actually do the copying/encoding
	available := make(chan bool, Concurrent)
	ack := make(chan bool)

	// handle each individual job
	go func() {
		section := ""
		for _, job := range script {
			// get a slot
			available <- true

			pieces := TargetName.FindStringSubmatch(job.Target)
			if len(pieces) != 3 {
				panic("Bad file name in script: " + job.Target)
			}
			newsection, article := pieces[1], pieces[2]
			if newsection != section {
				section = newsection
				log.Print("Section: ", section)
			}

			go func(pair *Pair, article string) {
				log.Print("    ", article)

				// copy the file over
				if err := ioutil.WriteFile(pair.Target, contents[pair.Source], 0644); err != nil {
					log.Fatalf("Error writing file %s: %v", pair.Target, err)
				}

				/*
					// scale the volume
					cmd := exec.Command(
						"mp3gain",
						"-g", ScalingFactor,
						pair.Target)
					if err := cmd.Run(); err != nil {
						log.Fatal("Scaling volume for ", pair.Target, ": ", err)
					}
				*/

				// sync
				fp, err := os.Open(pair.Target)
				if err != nil {
					log.Fatal("Opening file: ", err)
				}
				defer fp.Close()
				if err = fp.Sync(); err != nil {
					log.Fatal("Syncing file: ", err)
				}

				ack <- true
				<-available
			}(job, article)

			// pause a bit so it can (probably) get started
			// before we launch the next one
			// this helps keep the files in the file system mostly
			// in play order
			time.Sleep(50 * time.Millisecond)
		}

	}()

	// wait until all jobs are finished
	for _, _ = range script {
		<-ack
	}

	// final sync
	syscall.Sync()
	syscall.Sync()

	elapsed := time.Since(start)
	log.Printf("Finished in %v", elapsed-elapsed%time.Second)
}
