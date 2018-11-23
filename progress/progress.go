package progress

import (
	"fmt"
	"io"
	"time"
)

type Report struct {
	Completed float64 // 0.0 <= Completed <= 1.0
	Err       error
}

type Reader func(reader io.Reader, ch chan<- Report)

func bar(progress float64) string {
	done := "####################"
	notDone := "...................."
	doneCount := int(progress * 20)
	if doneCount > 20 {
		doneCount = 20
	}
	return fmt.Sprintf("[%s%s] %3.1f%%", done[:doneCount], notDone[doneCount:], progress*100)
}

func PrintProgress(name string, ch <-chan Report) error {
	prefix := fmt.Sprintf("%-8s", name)
	fmt.Print(prefix)
	spinner := []string{".", " "}
	i := 0
	start := time.Now()
	for p := range ch {
		if p.Err != nil {
			fmt.Printf("\n")
			return p.Err
		}
		fmt.Printf("<%#v>", p)
		timePassed := time.Since(start)
		estimatedTotalTime := time.Duration(float64(timePassed.Nanoseconds()) / p.Completed)
		timeLeft := estimatedTotalTime - timePassed
		fmt.Printf("\r\033[K%s%s %s ETA %s", prefix, bar(p.Completed), spinner[i], timeLeft.Truncate(time.Second))
		i = (i + 1) % len(spinner)
	}
	fmt.Printf("\r\033[K%s%s   Total time %s\n", prefix, bar(1.0), time.Since(start).Truncate(time.Second))
	return nil
}
