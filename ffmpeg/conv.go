package ffmpeg

import (
	"context"
	"fmt"
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
	cmd = append(cmd, "-pass", "1")
	fmt.Printf("$ %s\n", strings.Join(cmd, " "))
}

func Pass2(ctx context.Context, fi FileInfo) {
	fmt.Printf("Pass 2:\n")
	cmd := []string{}
	commonArgs(&cmd)
	cmd = append(cmd, "-pass", "2")
	fmt.Printf("$ %s\n", strings.Join(cmd, " "))
}
