package ffmpeg

import (
	"fmt"
	"time"
)

func parseDuration(ts string) time.Duration {
	var hours, minutes, seconds, hundredths time.Duration
	n, err := fmt.Sscanf(ts, "%d:%d:%d.%d", &hours, &minutes, &seconds, &hundredths)
	if err != nil || n != 4 {
		return time.Duration(0)
	}
	return time.Duration(hours*time.Hour +
		minutes*time.Minute +
		seconds*time.Second +
		hundredths*10*time.Millisecond)
}
