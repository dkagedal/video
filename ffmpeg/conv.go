package ffmpeg

import (
	"context"
	"fmt"
	"log"
	"strings"
	"video/stream"
)

func commonArgs(cmd *[]string) {
	*cmd = append(*cmd,
		"ffmpeg",
		// Process all streams.
		"-map", "0",
		// Copy streams by default, eg subtitles.
		"-c", "copy",
		// Keyframe spacing 240 frames.
		"-g", "240",
		// Use the vp9 codec for video.
		"-c:v", "libvpx-vp9",
	)
}

func normalizeResolution(resolution string) string {
	return resolution
}

func streamArgs(cmd *[]string, s stream.Stream, pass int) {
	switch s.Typ {
	case stream.Video:
		var targetrate string
		var minrate string
		var maxrate string
		var crf string
		var tileColumns string
		var threads string
		var speed string
		switch normalizeResolution(s.Resolution) {
		case "1920x1080":
			// Target bitrate 2Mbps, with constrained quality:
			targetrate = "2000k"
			crf = "31"
			minrate = "1000k"
			maxrate = "3000k"
			// Recommended settings for faster encoding:
			tileColumns = "2"
			threads = "8"
			speed = "2"
		case "720x576":
			// PAL is not listed on the Google VOD recommendations site, so I interpolate a bit
			// Target bitrate 2Mbps, with constrained quality:
			targetrate = "1000k"
			crf = "33"
			minrate = "400k"
			maxrate = "1200k"
			// Recommended settings for faster encoding:
			tileColumns = "1"
			threads = "4"
			speed = "2"
		case "720x480":
			// The Google VOD recommendations site only lists 640x480, but we'll treat this the same.
			// Target bitrate 2Mbps, with constrained quality:
			targetrate = "750k"
			crf = "33"
			minrate = "375k"
			maxrate = "1088k"
			// Recommended settings for faster encoding:
			tileColumns = "1"
			threads = "4"
			speed = "1"
		default:
			log.Fatal("Unknown resolution: ", s.Resolution)
		}
		if pass == 1 {
			speed = "4"
		}
		*cmd = append(*cmd,
			// Target bitrate 2Mbps, with constrained quality:
			"-b:v", targetrate, "-crf", crf, "-minrate", minrate, "-maxrate", maxrate,
			// Recommended settings for faster encoding:
			"-tile-columns", tileColumns, "-threads", threads,
			"-speed", speed)

	case stream.Audio:

	case stream.Subtitle:
	}
}

func findSingleVideoStream(fi FileInfo) *stream.Stream {
	ss := stream.Filter(stream.Video, fi.Streams)
	if len(ss) == 0 {
		return nil
	}
	if len(ss) > 1 {
		panic("More than one video stream")
	}
	return &ss[0]
}

func Pass1(ctx context.Context, fi FileInfo) {
	fmt.Printf("Pass 1:\n")
	cmd := []string{}
	commonArgs(&cmd)
	for _, s := range fi.Streams {
		streamArgs(&cmd, s, 1)
	}
	cmd = append(cmd, "-pass", "1", "-f", "matroska", "-y", "/dev/null")
	fmt.Printf("$ %s\n", strings.Join(cmd, " "))
}

func Pass2(ctx context.Context, fi FileInfo, destination string) {
	fmt.Printf("Pass 2:\n")
	cmd := []string{}
	commonArgs(&cmd)
	for _, s := range fi.Streams {
		streamArgs(&cmd, s, 2)
	}
	cmd = append(cmd, "-pass", "2", destination)
	fmt.Printf("$ %s\n", strings.Join(cmd, " "))
}
