package ffmpeg

import (
	"fmt"
)

// duration expressed in milliseconds
type Duration int64

func parseDuration(ts string) Duration {
	var hours, minutes, seconds, hundredths int64
	n, err := fmt.Sscanf(ts, "%d:%d:%d.%d", &hours, &minutes, &seconds, &hundredths)
	if err != nil || n != 4 {
		return Duration(0)
	}
	return Duration(hours*3600000 +
		minutes*60000 +
		seconds*1000 +
		hundredths*10)
}

func (d Duration) String() string {
	d /= 10
	hundredths := d % 100
	d /= 100
	seconds := d % 60
	d /= 60
	minutes := d % 60
	hours := d / 60
	return fmt.Sprintf("%02dh%02dm%02ds%02d", hours, minutes, seconds, hundredths)
}
