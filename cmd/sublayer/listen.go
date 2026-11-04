package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/translate"
	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

const (
	whisperPath    = "/inference"
	librePath      = "/translate"
	serverTimeout  = 30 * time.Second
	waitingPhrases = 3
)

type hearing struct {
	language string // empty leaves it to the settings
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
	language := flags.String("lang", "", "")

	if flags.Parse(args) != nil || flags.NArg() > 1 {
		return hearing{}, errUsage
	}

	if *language != "" && !slices.Contains(settings.Languages, *language) {
		return hearing{}, errUsage
	}

	return hearing{language: *language, file: flags.Arg(0)}, nil
}

func (h hearing) over(s settings.Settings) settings.Settings {
	if h.language != "" {
		s.Language = h.language
	}

	return s
}

func settle(args []string, m machine) (hearing, settings.Settings, error) {
	asked, err := parseListen(args)
	if err != nil {
		return hearing{}, settings.Settings{}, err
	}

	s, err := m.settings()
	if err != nil {
		return hearing{}, settings.Settings{}, err
	}

	return asked, asked.over(s), nil
}

func listen(args []string, m machine) error {
	asked, s, err := settle(args, m)
	if err != nil {
		return err
	}

	if asked.file != "" {
		return listenFile(asked.file, s, m)
	}

	return listenSpeakers(s, m)
}

func listenSpeakers(s settings.Settings, m machine) error {
	stop, err := serveUntilInterrupted(m, s)
	if err != nil {
		return err
	}
	defer stop()

	source, err := capture.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	defer closeOnInterrupt(source)()

	fmt.Fprintln(os.Stderr, "listening to the speakers; each phrase shows its times and text with the translation under it, and in brackets how long each took; press Ctrl+C to stop")

	v := voicesFor(s)

	return interpret(pipeline.Config{
		Source:      source,
		Rate:        capture.Rate,
		Waiting:     waitingPhrases,
		Transcriber: v.whisper(s.Language),
		Translator:  v.libre(s.Language, s.Target),
		Report:      show,
	}, s)
}

func listenFile(path string, s settings.Settings, m machine) error {
	file, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	samples, rate, err := wav.Decode(file)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	stop, err := serveUntilInterrupted(m, s)
	if err != nil {
		return err
	}
	defer stop()

	v := voicesFor(s)

	return interpret(pipeline.Config{
		Source:      &recording{samples: samples},
		Rate:        rate,
		Transcriber: v.whisper(s.Language),
		Translator:  v.libre(s.Language, s.Target),
		Report:      show,
	}, s)
}

func interpret(c pipeline.Config, s settings.Settings) error {
	c.Phrases = s.Phrases.Config()

	return unlessClosed(pipeline.Run(context.Background(), c))
}

type voices struct {
	whisper func(language string) stt.Transcriber
	libre   func(source, target string) translate.Translator
}

func voicesFor(s settings.Settings) voices {
	client := localClient()

	return voices{
		whisper: func(language string) stt.Transcriber {
			return stt.NewWhisper(stt.Config{URL: endpoint(s.Whisper.Server, whisperPath), Language: language}, client)
		},
		libre: func(source, target string) translate.Translator {
			return translate.NewLibre(translate.Config{URL: endpoint(s.Libre, librePath), Source: source, Target: target}, client)
		},
	}
}

func endpoint(server settings.Server, path string) string {
	return (&url.URL{Scheme: "http", Host: server.Address, Path: path}).String()
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
