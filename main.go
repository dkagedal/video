package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"video/ffmpeg"
	"video/stream"
)

func convert(ctx context.Context, filename string) {
	fmt.Printf("Converting %s\n", filename)
	info, err := ffmpeg.Probe(filename)
	if err != nil {
		log.Fatal(err)
	}
	stream.PrintTable(info.Streams)
	ffmpeg.Pass1(ctx, info)
	ffmpeg.Pass2(ctx, info)
}

func main() {
	flag.Parse()
	ctx := context.Background()
	for _, f := range flag.Args() {
		convert(ctx, f)
	}
}
