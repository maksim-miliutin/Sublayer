package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

func listen() error {
	source, err := capture.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	defer closeOnInterrupt(source)()

	fmt.Fprintln(os.Stderr, "listening to the speakers for phrases; press Ctrl+C to stop")

	s := vad.New(capture.Rate, vad.Defaults())
	for {
		chunk, err := source.Read()
		if err != nil {
			report(s.Flush())

			if errors.Is(err, capture.ErrClosed) {
				return nil
			}

			return err
		}

		report(s.Feed(chunk))
	}
}

func listenFile(path string) error {
	file, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	samples, rate, err := wav.Decode(file)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	s := vad.New(rate, vad.Defaults())
	report(append(s.Feed(samples), s.Flush()...))

	return nil
}

func report(phrases []vad.Phrase) {
	for _, p := range phrases {
		fmt.Println(describe(p))
	}
}

func describe(p vad.Phrase) string {
	return fmt.Sprintf("%s  %s  %5.2f s", stamp(p.Start), stamp(p.End), (p.End - p.Start).Seconds())
}

func stamp(d time.Duration) string {
	centis := d.Round(10*time.Millisecond) / (10 * time.Millisecond)

	return fmt.Sprintf("%02d:%02d.%02d", centis/6000, centis/100%60, centis%100)
}
