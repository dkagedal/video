package ffmpeg

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"video/stream"
)

var (
	progressRe = regexp.MustCompile(`frame=\s*(\d+) fps=\s*(\d+(?:\.\d+)?) q=(\d+\.\d+) size=\s*(\d+kB) time=(\d\d:\d\d:\d\d\.\d\d) bitrate=\s*(\d+\.\d+)kbits/s speed=(\d+\.\d+)x\s*\r`)
)

func videoQualityArgs(cmd *[]string, fi *FileInfo, pass int) {
	var targetrate string
	var minrate string
	var maxrate string
	var crf string
	var tileColumns string
	var threads string
	var speed string
	resolution := fi.fileResolution().Normalized()
	switch resolution {
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
		log.Fatal("Unknown resolution: ", resolution)
	}
	if pass == 1 {
		speed = "4"
	}
	*cmd = append(*cmd,
		// Keyframe spacing 240 frames.
		"-g", "240",
		// Use the vp9 codec for video.
		"-c:v", "libvpx-vp9",
		// Target bitrate 2Mbps, with constrained quality:
		"-b:v", targetrate, "-crf", crf, "-minrate", minrate, "-maxrate", maxrate,
		// Recommended settings for faster encoding:
		"-tile-columns", tileColumns, "-threads", threads,
		"-speed", speed,
	)
}

func hashString(s string) uint32 {
	alg := fnv.New32a()
	alg.Write([]byte(s))
	return alg.Sum32()
}

func tmpFilePrefix(fi *FileInfo) string {
	return fmt.Sprintf("vp9-%d", hashString(fi.Filename))
}

type progress struct {
	timestamp time.Duration
	err       error
}

func readProgress(reader io.Reader, ch chan<- progress) {
	buffer := make([]byte, 0, 4096)
	for {
		if sub := progressRe.FindSubmatchIndex(buffer); sub != nil {
			match := func(i int) string {
				return string(buffer[sub[i*2]:sub[i*2+1]])
			}
			p := progress{
				timestamp: parseDuration(match(5)),
			}
			ch <- p
			buffer = buffer[sub[1]:]
		} else {
			if len(buffer) > 2048 {
				buffer = buffer[1024:]
			}
			if cap(buffer)-len(buffer) < 256 {
				newbuffer := make([]byte, len(buffer), 4096)
				copy(newbuffer, buffer)
				buffer = newbuffer
			}
			readbuf := buffer[len(buffer):cap(buffer)]
			n, err := reader.Read(readbuf)
			readbuf = readbuf[:n]
			// fmt.Printf("Read %d bytes: %#v\n", n, string(readbuf))
			buffer = append(buffer, readbuf...) // assume this is smart
			if n == 0 {
				if err != io.EOF {
					ch <- progress{err: err}
				}
				close(ch)
				return
			}
		}
		// fmt.Printf("Buffer: %#v\n", string(buffer))
	}
}

func progressBar(progress float64) string {
	done := "####################"
	notDone := "...................."
	doneCount := int(progress * 20)
	return fmt.Sprintf("[%s%s] %3d%%", done[:doneCount], notDone[doneCount:], int(progress*100))
}

func runWithProgress(prefix string, fi FileInfo, cmd *exec.Cmd) error {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("\n")
		return err
	}
	if err = cmd.Start(); err != nil {
		fmt.Printf("\n")
		return err
	}
	ch := make(chan progress)
	go readProgress(stderr, ch)
	spinner := []string{".", " "}
	i := 0
	start := time.Now()
	for p := range ch {
		if p.err != nil {
			fmt.Printf("\n")
			return p.err
		}
		progress := float64(p.timestamp) / float64(fi.Length)
		eta := time.Duration(float64(time.Since(start).Nanoseconds()) / progress)
		fmt.Printf("\r\033[K%s%s %s ETA %s", prefix, progressBar(progress), spinner[i], eta.Truncate(time.Second))
		i = (i + 1) % len(spinner)
	}
	fmt.Printf("\r\033[K%s%s\n", prefix, progressBar(1.0))
	return nil
}

func FindCrop(ctx context.Context, fi FileInfo) error {
	fmt.Printf("Crop:   ")
	args := []string{
		"-i", fi.Filename,
		"-t", "10", // stop after 10 seconds
		"-vf", "cropdetect",
		"-f", "null",
		"-",
	}
	fmt.Printf("$ ffmpeg '%s'\n", strings.Join(args, "' '"))
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	//return runWithProgress("Crop:   ", fi, cmd)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Pass1(ctx context.Context, fi FileInfo) error {
	fmt.Printf("Pass 1: ")
	args := []string{
		"-i", fi.Filename,
		// Process all streams.
		"-map", "0",
		// Copy streams by default, eg subtitles.
		"-c", "copy",
	}
	videoQualityArgs(&args, &fi, 1)
	args = append(args, "-passlogfile", tmpFilePrefix(&fi), "-pass", "1", "-f", "matroska", "-y", "/dev/null")
	// fmt.Printf("$ ffmpeg '%s'\n", strings.Join(args, "' '"))
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	return runWithProgress("Pass 1: ", fi, cmd)
}

func Pass2(ctx context.Context, fi FileInfo, destination string) error {
	fmt.Printf("Pass 2: ")
	args := []string{
		"-i", fi.Filename,
		// Process all streams.
		"-map", "0",
		// Copy streams by default, eg subtitles.
		"-c", "copy",
	}
	videoQualityArgs(&args, &fi, 2)
	args = append(
		args,
		// Use OPUS for audio.
		"-c:a", "libopus",
	)
	for _, s := range fi.Streams {
		if s.Typ == stream.Audio && s.Channels == "5.1(side)" {
			// There is currently a bug (https://trac.ffmpeg.org/ticket/5718) in
			// ffmpeg/libopus that makes it fail if the input channel layout is
			// "5.1(side)". This hack adds a filter that sets the output layout
			// to "5.1" in those audio streams.
			args = append(args, "-filter:"+s.Id, "aformat=channel_layouts=5.1")
		}
	}
	args = append(args, "-passlogfile", tmpFilePrefix(&fi), "-pass", "2", destination)
	// fmt.Printf("$ ffmpeg '%s'\n", strings.Join(args, "' '"))
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	return runWithProgress("Pass 2: ", fi, cmd)
}
