package ffmpeg

import (
	"bufio"
	"os/exec"
	"regexp"
	"strings"
	"video/stream"
)

var (
	streamRe     = regexp.MustCompile(`^ *Stream #\d+:(\d+)\((...)\): (\S+): (.*)$`)
	resolutionRe = regexp.MustCompile(`^(\d+x\d+)(?: \[.*\])$`)
	channelsRe   = regexp.MustCompile(`^(stereo|5.1(?:\(side\))?)$`)
)

type FileInfo struct {
	Filename string
	Streams  []stream.Stream
}

func Probe(filename string) (FileInfo, error) {
	info := FileInfo{
		Filename: filename,
		Streams:  make([]stream.Stream, 0),
	}
	cmd := exec.Command("ffprobe", filename)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return info, err
	}
	if err = cmd.Start(); err != nil {
		return info, err
	}
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		// fmt.Println("::: " + scanner.Text())
		if sub := streamRe.FindStringSubmatch(scanner.Text()); sub != nil {
			isdef := false
			if strings.HasSuffix(sub[4], " (default)") {
				isdef = true
				sub[4] = strings.TrimSuffix(sub[4], " (default)")
			}

			codecinfo := strings.Split(sub[4], ", ")
			s := stream.Stream{
				Id:        sub[1],
				Typ:       stream.Type(sub[3]),
				Lang:      sub[2],
				Codec:     codecinfo[0],
				IsDefault: isdef,
				Params:    []string{},
			}
			// fmt.Printf("%q\n", sub)
			// fmt.Printf("Codec params: %q\n", codecparams)

			for _, p := range codecinfo[1:] {
				if m := resolutionRe.FindStringSubmatch(p); m != nil {
					s.Resolution = m[1]
				} else if m := channelsRe.FindStringSubmatch(p); m != nil {
					s.Channels = m[1]
				} else {
					s.Params = append(s.Params, p)
				}
			}
			info.Streams = append(info.Streams, s)
		}
	}
	return info, nil
}
