package pipeline

import (
	"context"
	"errors"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

var ErrSkipped = errors.New("pipeline: skipped while recognition was behind")

type Config struct {
	Source      capture.Source
	Rate        int // samples per second the source hands out
	Phrases     vad.Config
	Transcriber stt.Transcriber
	Waiting     int // phrases kept waiting while one is recognized; zero keeps every one
	Report      func(Result)
}

type Result struct {
	Phrase vad.Phrase
	Text   string
	Took   time.Duration
	Err    error
}

func Run(ctx context.Context, c Config) error {
	waiting := newQueue[vad.Phrase](c.Waiting)

	recognized := make(chan struct{})
	go func() {
		recognize(ctx, c, waiting)
		close(recognized)
	}()

	err := segment(c, waiting)
	waiting.close()
	<-recognized

	return err
}

func segment(c Config, waiting *queue[vad.Phrase]) error {
	s := vad.New(c.Rate, c.Phrases)
	for {
		chunk, err := c.Source.Read()
		if err != nil {
			waiting.push(s.Flush()...)

			return err
		}

		waiting.push(s.Feed(chunk)...)
	}
}

func recognize(ctx context.Context, c Config, waiting *queue[vad.Phrase]) {
	for {
		p, skipped, ok := waiting.pop()
		skip(c.Report, skipped)

		if !ok {
			return
		}

		began := time.Now()
		text, err := c.Transcriber.Transcribe(ctx, p.Samples, c.Rate)
		c.Report(Result{Phrase: p, Text: text, Took: time.Since(began), Err: err})
	}
}

func skip(report func(Result), phrases []vad.Phrase) {
	for _, p := range phrases {
		report(Result{Phrase: p, Err: ErrSkipped})
	}
}
