package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"video/ffmpeg"
	"video/stream"
)

func checkInput(info ffmpeg.FileInfo) {
	nvideo := 0
	for _, s := range info.Streams {
		if s.Typ == stream.Video {
			fmt.Printf("Codec: %s\n", s.Codec)
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

func selectDestination(destDir string, source string) string {
	sourceDir := path.Dir(source)
	filename := path.Base(source)
	if destDir == "" {
		destDir = path.Join(sourceDir, "vp9")

	}
	destination := path.Join(destDir, fixExtension(filename))

	files, err := ioutil.ReadDir(destDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.Name() == filename {
			log.Fatal("File %v already exists", destination)
		}
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
	fmt.Printf("Duration: %s\n", info.Duration)
	checkInput(info)

	destination := selectDestination(flag.Arg(1), source)
	fmt.Printf("Saving to %s\n", destination)
	ffmpeg.Pass1(ctx, info)
	ffmpeg.Pass2(ctx, info, destination)
}
