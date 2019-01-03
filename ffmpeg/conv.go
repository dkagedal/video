package ffmpeg

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"video/progress"
	"video/stream"
)

var (
	showCmdFlag = flag.Bool("showcmd", false, "Show the commands being run.")

	cropRe     = regexp.MustCompile(`crop=(\d+:\d+:\d+:\d+)`)
	progressRe = regexp.MustCompile(`frame=\s*(\d+) ` +
		`fps=\s*(\S+) ` +
		`q=\s*(\S+) ` +
		`size=\s*(\S+) ` +
		`time=(\d\d:\d\d:\d\d\.\d\d) ` +
		`bitrate=\s*(\S+) ` +
		`speed=\s*\S*x\s*\r`)
)

func videoQualityArgs(cmd *[]string, fi *FileInfo, cropArg string, pass int) {
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
	if cropArg != "" {
		*cmd = append(*cmd,
			"-filter:v", "crop="+cropArg,
		)
	}
}

func hashString(s string) uint32 {
	alg := fnv.New32a()
	alg.Write([]byte(s))
	return alg.Sum32()
}

func readConversionProgress(reader io.Reader, fi FileInfo, ch chan<- progress.Report) {
	buffer := make([]byte, 0, 4096)
	for {
		if sub := progressRe.FindSubmatchIndex(buffer); sub != nil {
			match := func(i int) string {
				return string(buffer[sub[i*2]:sub[i*2+1]])
			}
			timestamp := parseDuration(match(5))
			// fmt.Printf("reporting %s (%.2f)", timestamp, float64(timestamp)/float64(fi.Length))
			ch <- progress.Report{
				Completed: float64(timestamp) / float64(fi.Length),
			}
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
					ch <- progress.Report{Err: err}
				}
				return
			}
		}
		// fmt.Printf("Buffer: %#v\n", string(buffer))
	}
}

func start(cmd *exec.Cmd) (io.Reader, error) {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("\n")
		return nil, err
	}
	cmdlog, err := ioutil.TempFile("", "ffmpeg-*.log")
	if err != nil {
		fmt.Printf("\n")
		return nil, fmt.Errorf("Failed to open log file: %v", err)
	}
	if err = cmd.Start(); err != nil {
		fmt.Printf("\n")
		return nil, err
	}
	return io.TeeReader(stderr, cmdlog), nil
}

func FindCrop(ctx context.Context, fi FileInfo, cropArg *string, ch chan<- progress.Report) {
	defer close(ch)
	ch <- progress.Report{Completed: 0.0}
	args := []string{
		"-ss", "00:01:00", // Skip one minute into the movie
		"-i", fi.Filename,
		"-t", "10", // stop after 30 seconds
		"-vf", "cropdetect",
		"-f", "null",
		"-",
	}
	if *showCmdFlag {
		fmt.Printf("$ ffmpeg '%s'\n", strings.Join(args, "' '"))
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := start(cmd)
	if err != nil {
		ch <- progress.Report{Err: err}
		return
	}

	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		if sub := cropRe.FindStringSubmatch(scanner.Text()); sub != nil {
			*cropArg = sub[1]
			ch <- progress.Report{Status: *cropArg}
		}
	}
	if err = scanner.Err(); err != nil {
		ch <- progress.Report{Err: err}
	}
	if err = cmd.Wait(); err != nil {
		ch <- progress.Report{Err: err}
	}
}

// Pass1 runs the first pass of a two pass transcoding.
func Pass1(ctx context.Context, fi FileInfo, cropArg string, ch chan<- progress.Report) {
	defer close(ch)
	ch <- progress.Report{Completed: 0.0}
	args := []string{
		"-i", fi.Filename,
		// Process all streams.
		"-map", "0",
		// Copy streams by default, eg subtitles.
		"-c", "copy",
	}
	videoQualityArgs(&args, &fi, cropArg, 1)
	for _, s := range fi.Streams {
		if s.ShouldSkip() {
			args = append(args, "-map", "-0:"+s.Id)
		}
	}
	args = append(args, "-passlogfile", fi.passlogfile(), "-pass", "1", "-f", "matroska", "-y", "/dev/null")
	if *showCmdFlag {
		fmt.Printf("$ ffmpeg '%s'\n", strings.Join(args, "' '"))
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := start(cmd)
	if err != nil {
		ch <- progress.Report{Err: err}
		return
	}
	readConversionProgress(output, fi, ch)
	if err = cmd.Wait(); err != nil {
		ch <- progress.Report{Err: err}
	}
}

// Pass2 runs the second pass of a two pass transcoding.
func Pass2(ctx context.Context, fi FileInfo, destination string, cropArg string, ch chan<- progress.Report) {
	defer close(ch)
	ch <- progress.Report{Completed: 0.0}
	args := []string{
		"-i", fi.Filename,
		// Process all streams.
		"-map", "0",
		// Copy streams by default, eg subtitles.
		"-c", "copy",
	}
	videoQualityArgs(&args, &fi, cropArg, 2)
	args = append(
		args,
		// Use OPUS for audio.
		"-c:a", "libopus",
	)
	for _, s := range fi.Streams {
		if s.ShouldSkip() {
			args = append(args, "-map", "-0:"+s.Id)
		}
		if s.Typ == stream.Audio && s.Channels == "5.1(side)" {
			// There is currently a bug (https://trac.ffmpeg.org/ticket/5718) in
			// ffmpeg/libopus that makes it fail if the input channel layout is
			// "5.1(side)". This hack adds a filter that sets the output layout
			// to "5.1" in those audio streams.
			args = append(args, "-filter:"+s.Id, "aformat=channel_layouts=5.1")
		}
	}
	args = append(args, "-passlogfile", fi.passlogfile(), "-pass", "2", destination)
	if *showCmdFlag {
		fmt.Printf("$ ffmpeg '%s'\n", strings.Join(args, "' '"))
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := start(cmd)
	if err != nil {
		ch <- progress.Report{Err: err}
		return
	}
	readConversionProgress(output, fi, ch)
	if err = cmd.Wait(); err != nil {
		ch <- progress.Report{Err: err}
	}
}
