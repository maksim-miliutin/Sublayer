package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

func record(path string) error {
	source, err := capture.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	defer closeOnInterrupt(source)()

	fmt.Fprintln(os.Stderr, "recording what the speakers play; press Ctrl+C to stop")
	began := time.Now()

	samples, readErr := collect(source)
	elapsed := time.Since(began)

	if len(samples) == 0 {
		fmt.Fprintln(os.Stderr, "nothing was recorded, no file written")

		return readErr
	}

	if err := os.WriteFile(path, wav.Encode(samples, capture.Rate), 0o644); err != nil {
		return errors.Join(readErr, err)
	}

	sound := float64(len(samples)) / capture.Rate
	fmt.Fprintf(os.Stderr, "%s: %.2f s of sound over %.2f s of recording\n", path, sound, elapsed.Seconds())

	return readErr
}

func collect(source capture.Source) ([]float32, error) {
	var samples []float32
	for {
		chunk, err := source.Read()
		if err != nil {
			return samples, unlessClosed(err)
		}

		samples = append(samples, chunk...)
	}
}
