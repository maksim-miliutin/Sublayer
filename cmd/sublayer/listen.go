package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/translate"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

const (
	whisperURL     = "http://127.0.0.1:8178/inference"
	libreURL       = "http://127.0.0.1:5000/translate"
	serverTimeout  = 30 * time.Second
	waitingPhrases = 3
	translateInto  = "ru"
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

	fmt.Fprintln(os.Stderr, "listening to the speakers; each phrase shows its times and text with the translation under it, and in brackets how long each took; press Ctrl+C to stop")

	return interpret(pipeline.Config{Source: source, Rate: capture.Rate, Waiting: waitingPhrases, Report: show}, language)
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

	return interpret(pipeline.Config{Source: &recording{samples: samples}, Rate: rate, Report: show}, language)
}

func interpret(c pipeline.Config, language string) error {
	client := localClient()

	c.Phrases = vad.Defaults()
	c.Transcriber = stt.NewWhisper(stt.Config{URL: whisperURL, Language: language}, client)
	c.Translator = translate.NewLibre(translate.Config{URL: libreURL, Source: language, Target: translateInto}, client)

	return unlessClosed(pipeline.Run(context.Background(), c))
}

// Unlike the default client this one never takes a proxy from the environment, so speech
// and its text go to the servers named here and nowhere else, wherever those servers run.
func localClient() *http.Client {
	return &http.Client{Transport: &http.Transport{}, Timeout: serverTimeout}
}

func show(r pipeline.Result) {
	if r.Text == "" && r.Err != nil {
		fmt.Fprintln(os.Stderr, describe(r))

		return
	}

	fmt.Println(describe(r))
}

func describe(r pipeline.Result) string {
	p := r.Phrase
	times := fmt.Sprintf("%s  %s  %5.2f s", stamp(p.Start), stamp(p.End), (p.End - p.Start).Seconds())

	if r.Text == "" {
		return times + "  " + unheard(r)
	}

	return fmt.Sprintf("%s  (%.2f s)  %s\n%s  %s", times, r.Took.Recognize.Seconds(), r.Text, strings.Repeat(" ", len(times)), translated(r))
}

func unheard(r pipeline.Result) string {
	if r.Err != nil {
		return r.Err.Error()
	}

	return fmt.Sprintf("(%.2f s)", r.Took.Recognize.Seconds())
}

func translated(r pipeline.Result) string {
	if r.Err != nil {
		return r.Err.Error()
	}

	return fmt.Sprintf("(%.2f s)  %s", r.Took.Translate.Seconds(), r.Translation)
}

func stamp(d time.Duration) string {
	centis := d.Round(10*time.Millisecond) / (10 * time.Millisecond)

	return fmt.Sprintf("%02d:%02d.%02d", centis/6000, centis/100%60, centis%100)
}
