package pipeline

import (
	"context"
	"errors"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/translate"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

var ErrSkipped = errors.New("pipeline: skipped while recognition was behind")

type Config struct {
	Source      capture.Source
	Rate        int // samples per second the source hands out
	Phrases     vad.Config
	Transcriber stt.Transcriber
	Translator  translate.Translator
	Waiting     int              // phrases kept waiting while one is recognized and translated; zero keeps every one
	Recognizing func(vad.Phrase) // told as each phrase goes to recognition; nil when nobody asks
	Report      func(Result)
}

type Result struct {
	Phrase      vad.Phrase
	Text        string
	Translation string
	Took        Timing
	Err         error
}

type Timing struct {
	Recognize time.Duration
	Translate time.Duration
}

func Run(ctx context.Context, c Config) error {
	if c.Recognizing == nil {
		c.Recognizing = func(vad.Phrase) {}
	}

	waiting := newQueue[vad.Phrase](c.Waiting)

	subtitled := make(chan struct{})
	go func() {
		subtitle(ctx, c, waiting)
		close(subtitled)
	}()

	err := segment(c, waiting)
	waiting.close()
	<-subtitled

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

func subtitle(ctx context.Context, c Config, waiting *queue[vad.Phrase]) {
	for {
		p, skipped, ok := waiting.pop()
		skip(c.Report, skipped)

		if !ok {
			return
		}

		c.Recognizing(p)
		c.Report(pair(ctx, c, p))
	}
}

func pair(ctx context.Context, c Config, p vad.Phrase) Result {
	r := Result{Phrase: p}

	began := time.Now()
	text, err := c.Transcriber.Transcribe(ctx, p.Samples, c.Rate)
	r.Took.Recognize = time.Since(began)

	if err != nil || text == "" {
		r.Err = err

		return r
	}
	r.Text = text

	began = time.Now()
	translation, err := c.Translator.Translate(ctx, text)
	r.Took.Translate = time.Since(began)

	if err != nil {
		r.Err = err

		return r
	}
	r.Translation = translation

	return r
}

func skip(report func(Result), phrases []vad.Phrase) {
	for _, p := range phrases {
		report(Result{Phrase: p, Err: ErrSkipped})
	}
}
