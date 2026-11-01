package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

const (
	whisperURL     = "http://127.0.0.1:8178/inference"
	whisperTimeout = 30 * time.Second
	waitingPhrases = 3
)

// whisper-server takes a language it does not know without complaint and hears nonsense,
// so only the languages Sublayer is meant for get through.
var languages = []string{"auto", "fr", "en"}

type hearing struct {
	language string
	file     string
}

type recording struct {
	samples []float32
}

func (r *recording) Read() ([]float32, error) {
	if r.samples == nil {
		return nil, capture.ErrClosed
	}

	samples := r.samples
	r.samples = nil

	return samples, nil
}

func (r *recording) Close() error {
	return nil
}

func parseListen(args []string) (hearing, error) {
	flags := flag.NewFlagSet("listen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	language := flags.String("lang", "auto", "")

	if flags.Parse(args) != nil || flags.NArg() > 1 || !slices.Contains(languages, *language) {
		return hearing{}, errUsage
	}

	return hearing{language: *language, file: flags.Arg(0)}, nil
}

func listen(args []string) error {
	h, err := parseListen(args)
	if err != nil {
		return err
	}

	if h.file != "" {
		return listenFile(h.file, h.language)
	}

	return listenSpeakers(h.language)
}

func listenSpeakers(language string) error {
	source, err := capture.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	defer closeOnInterrupt(source)()

	fmt.Fprintln(os.Stderr, "listening to the speakers; each phrase shows its start, end, length, the time whisper took and the text; press Ctrl+C to stop")

	return recognize(pipeline.Config{Source: source, Rate: capture.Rate, Waiting: waitingPhrases, Report: show}, language)
}

func listenFile(path, language string) error {
	file, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	samples, rate, err := wav.Decode(file)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return recognize(pipeline.Config{Source: &recording{samples: samples}, Rate: rate, Report: show}, language)
}

func recognize(c pipeline.Config, language string) error {
	// Unlike the default client this one never takes a proxy from the environment, so
	// speech goes to whisper-server and nowhere else, wherever the server runs.
	client := &http.Client{Transport: &http.Transport{}, Timeout: whisperTimeout}

	c.Phrases = vad.Defaults()
	c.Transcriber = stt.NewWhisper(stt.Config{URL: whisperURL, Language: language}, client)

	return unlessClosed(pipeline.Run(context.Background(), c))
}

func show(r pipeline.Result) {
	if r.Err != nil {
		fmt.Fprintln(os.Stderr, describe(r))

		return
	}

	fmt.Println(describe(r))
}

func describe(r pipeline.Result) string {
	p := r.Phrase
	times := fmt.Sprintf("%s  %s  %5.2f s", stamp(p.Start), stamp(p.End), (p.End - p.Start).Seconds())

	if r.Err != nil {
		return times + "  " + r.Err.Error()
	}

	heard := fmt.Sprintf("%s  (%.2f s)", times, r.Took.Seconds())
	if r.Text == "" {
		return heard
	}

	return heard + "  " + r.Text
}

func stamp(d time.Duration) string {
	centis := d.Round(10*time.Millisecond) / (10 * time.Millisecond)

	return fmt.Sprintf("%02d:%02d.%02d", centis/6000, centis/100%60, centis%100)
}
