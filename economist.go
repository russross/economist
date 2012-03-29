package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

const (
	Source        = "/tmp/ec"
	Target        = "/media/economist/ec"
	Concurrent    = 6
	ScalingFactor = "2" // volume scaling factor
)

var Zipfile = os.Getenv("HOME") + "/Downloads/*The*Economist*.zip"

var (
	IsSourceFile        = regexp.MustCompile(`^(\d+) (.*?) - (.*\.mp3)$`)
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
	// make sure we have the latest edition downloaded (and only it)
	ziplist, err := filepath.Glob(Zipfile)
	if err != nil {
		log.Fatal("Finding zip file: ", err)
	}
	if len(ziplist) != 1 {
		log.Fatal("Wrong number of zip files: ", len(ziplist))
	}
	zip := ziplist[0]
	_, filename := filepath.Split(zip)
	log.Print(filename)

	// wipe out last week
	log.Print("Removing last week's issue...")
	if err = os.RemoveAll(Source); err != nil {
		log.Fatal("Clearing ", Source, ": ", err)
	}

	// make and go to the source directory
	if err = os.MkdirAll(Source, 0755); err != nil {
		log.Fatal("Creating source directory: ", err)
	}
	if err = os.Chdir(Source); err != nil {
		log.Fatal("Changing to source directory: ", err)
	}

	// unzip this week
	log.Print("Unzipping this week's issue...")
	if err = exec.Command("unzip", "-q", zip).Run(); err != nil {
		log.Fatal("Unzipping file: ", err)
	}

	// blow away last week on the ЅD drive
	log.Print("Clearing last week from SD drive...")
	if err = os.RemoveAll(Target); err != nil {
		log.Fatal("Clearing SD drive: ", err)
	}
	if err = os.Mkdir(Target, 0755); err != nil {
		log.Fatal("Making target directory: ", err)
	}
	if err = exec.Command("sync").Run(); err != nil {
		log.Fatal("Syncing: ", err)
	}

	// kill section intros and rearrange into a directory per section
	files, err := filepath.Glob("*.mp3")
	if err != nil {
		log.Fatal("Getting list of mp3 files: ", err)
	}
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
	idle := make(chan bool, Concurrent)
	ack := make(chan bool)

	// fill the list of idle tokens
	for i := 0; i < Concurrent; i++ {
		idle <- true
	}

	// handle each individual job
	go func() {
		section := ""
		for _, job := range script {
			// get a token
			<-idle

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
				cmd := exec.Command(
					"lame",
					"--quiet",
					"--scale", ScalingFactor,
					pair.Source,
					pair.Target)
				if err := cmd.Run(); err != nil {
					log.Fatal("Encoding ", pair.Source, ": ", err)
				}

				fp, err := os.Open(pair.Target)
				if err != nil {
					log.Fatal("Opening file: ", err)
				}
				defer fp.Close()
				if err = fp.Sync(); err != nil {
					log.Fatal("Syncing file: ", err)
				}

				ack <- true
				idle <- true
			}(job, article)

			// pause a bit so it can (probably) get started
			// before we launch the next one
			// this helps keep the files in the file system mostly
			// in play order
			time.Sleep(1e8)
		}

	}()

	// wait until all jobs are finished
	for _, _ = range script {
		<-ack
	}

	// final sync
	if err = exec.Command("sync").Run(); err != nil {
		log.Fatal("Syncing: ", err)
	}
	if err = exec.Command("sync").Run(); err != nil {
		log.Fatal("Syncing: ", err)
	}

	// delete the source directory
	if err = os.RemoveAll(Source); err != nil {
		log.Fatal("Removing source directory: ", err)
	}
	log.Print("Finished")
}