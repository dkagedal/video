package stream

import (
	"fmt"
	"strings"
)

type Type string

const (
	Audio    Type = "Audio"
	Video         = "Video"
	Subtitle      = "Subtitle"
)

type ResolutionString string

type Stream struct {
	Id         string
	Typ        Type
	Lang       string
	Codec      string
	Resolution ResolutionString
	Channels   string
	Params     []string
	IsDefault  bool
}

const repeat = `-"-`

func (r ResolutionString) Normalized() ResolutionString {
	return r
}

func formatParams(params []string) string {
	return strings.Join(params, " / ")
}

func PrintTable(streams []Stream) {
	var last Stream
	for _, s := range streams {
		ps := s
		params := formatParams(ps.Params)
		if ps.Typ == last.Typ {
			ps.Typ = repeat
			if ps.Lang == last.Lang {
				ps.Lang = repeat
			}
			if ps.Resolution == last.Resolution {
				ps.Resolution = repeat
			}
			if ps.Channels == last.Channels {
				ps.Channels = repeat
			}
			if ps.Codec == last.Codec {
				ps.Codec = repeat
			}
			if params != "" && params == formatParams(last.Params) {
				params = repeat
			}
		}

		primaryinfo := ""
		if ps.Resolution != "" {
			primaryinfo = string(ps.Resolution)
		} else if ps.Channels != "" {
			primaryinfo = ps.Channels
		}
		fmt.Printf("%5v  %-10v  %-10v  %-20v %3s  %v\n",
			ps.Id, ps.Typ, primaryinfo, ps.Codec, ps.Lang, params)
		last = s
	}
}

func Filter(typ Type, in []Stream) []Stream {
	out := []Stream{}
	for _, s := range in {
		if s.Typ == typ {
			out = append(out, s)
		}
	}
	return out
}
