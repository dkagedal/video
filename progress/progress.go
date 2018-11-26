package progress

import (
	"fmt"
	"io"
	"time"
)

type Report struct {
	Completed float64 // 0.0 <= Completed <= 1.0
	Status    string
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
	status := ""
	for p := range ch {
		if p.Err != nil {
			fmt.Printf("\n")
			return p.Err
		}
		eta := "??"
		timePassed := time.Since(start)
		if timePassed > 5*time.Second {
			estimatedTotalTime := time.Duration(float64(timePassed.Nanoseconds()) / p.Completed)
			timeLeft := estimatedTotalTime - timePassed
			eta = timeLeft.Truncate(time.Second).String()
		}
		fmt.Printf("\r\033[K%s%s %s ETA %s %s", prefix, bar(p.Completed), spinner[i], eta, p.Status)
		i = (i + 1) % len(spinner)
		status = p.Status
	}
	fmt.Printf("\r\033[K%s%s   %s (%s)\n", prefix, bar(1.0), status, time.Since(start).Truncate(time.Second))
	return nil
}
