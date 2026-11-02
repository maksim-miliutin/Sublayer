package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/maksim-miliutin/Sublayer/internal/app"
	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

func subtitles(args []string, m machine) error {
	asked, s, err := settle(args, m)
	if err != nil {
		return err
	}

	if asked.file != "" {
		return errUsage
	}

	window, err := ui.Open()
	if err != nil {
		return err
	}

	source, err := capture.Open()
	if err != nil {
		return errors.Join(err, window.Close())
	}
	defer source.Close()
	defer closeOnInterrupt(source)()

	screen := app.NewSubtitles(app.Config{Window: window, Shown: s.Pairs, StopListening: func() { source.Close() }})

	fmt.Fprintln(os.Stderr, "listening to the speakers and showing subtitles at the bottom left of the screen, each phrase printed here too; press Ctrl+C to stop")

	err = interpret(pipeline.Config{
		Source:      source,
		Rate:        capture.Rate,
		Waiting:     waitingPhrases,
		Recognizing: screen.Recognizing,
		Report: func(r pipeline.Result) {
			screen.Report(r)
			show(r)
		},
	}, s)

	return errors.Join(err, screen.Close())
}
