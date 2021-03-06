package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"video/ffmpeg"
	"video/progress"
	"video/stream"
)

var (
	cropFlag    = flag.Bool("crop", false, "Crop the video to remove black bars.")
	restartFlag = flag.Bool("restart", false, "Run only pass 2.")
)

func checkInput(info ffmpeg.FileInfo) {
	nvideo := 0
	for _, s := range info.Streams {
		if s.ShouldSkip() {
			continue
		}
		if s.Typ == stream.Video {
			fmt.Printf("Video codec: %s\n", s.Codec)
			if nvideo > 0 {
				log.Fatal("Multiple video streams")
			}
			if strings.HasPrefix(s.Codec, "vp9 ") {
				log.Fatal("Already VP9")
			}
			nvideo += 1
		}
	}
	if nvideo == 0 {
		log.Fatal("No video")
	}
}

func fixExtension(filename string) string {
	if strings.HasSuffix(filename, ".mkv") {
		return filename
	}
	return filename[:len(filename)-4] + ".mkv"
}

func fileExists(filename string) bool {
	dirname, basename := filepath.Split(filename)
	if dirname == "" {
		dirname = "."
	}
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.Name() == basename {
			return true
		}
	}
	return false
}

func selectDestination(destDir string, source string) string {
	sourceDir, filename := filepath.Split(source)
	if destDir == "" {
		destDir = filepath.Join(sourceDir, "vp9")
	}
	destination := filepath.Join(destDir, fixExtension(filename))
	if fileExists(destination) {
		log.Fatal("File already exists: ", destination)
	}
	return destination
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n  %s SOURCE [ DESTINATIONDIR ]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	flag.Parse()
	if flag.NArg() < 1 || flag.NArg() > 2 {
		flag.Usage()
	}
	source := flag.Arg(0)
	if len(source) == 0 {
		flag.Usage()
	}

	ctx := context.Background()
	info, err := ffmpeg.Probe(ctx, source)
	if err != nil {
		log.Fatal(err)
	}

	stream.PrintTable(info.Streams)
	fmt.Printf("Duration: %s\n", info.Length)
	checkInput(info)

	destination := selectDestination(flag.Arg(1), source)
	fmt.Printf("Saving to %s\n", destination)

	pass1Logfile := info.Pass1Logfile()
	if *restartFlag {
		if !fileExists(pass1Logfile) {
			log.Fatalf("No convertion to restart (%s does not exist)", pass1Logfile)
		}
	} else {
		if fileExists(pass1Logfile) {
			log.Fatalf("Remove %s to restart conversion", pass1Logfile)
		}
	}

	cropArg := ""
	if *cropFlag {
		crop := make(chan progress.Report)
		go ffmpeg.FindCrop(ctx, info, &cropArg, crop)
		err = progress.PrintProgress("Crop", crop)
		if err != nil {
			log.Fatal(err)
		}
	}

	if !*restartFlag {
		pass1 := make(chan progress.Report)
		go ffmpeg.Pass1(ctx, info, cropArg, pass1)
		err = progress.PrintProgress("Pass 1", pass1)
		if err != nil {
			log.Fatal(err)
		}
	}

	pass2 := make(chan progress.Report)
	go ffmpeg.Pass2(ctx, info, destination, cropArg, pass2)
	if err = progress.PrintProgress("Pass 2", pass2); err != nil {
		log.Fatal(err)
	}
}
