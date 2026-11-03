package main

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/translate"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

func TestStampWritesMinutesSecondsAndHundredths(t *testing.T) {
	cases := map[time.Duration]string{
		0:                         "00:00.00",
		1800 * time.Millisecond:   "00:01.80",
		61234 * time.Millisecond:  "01:01.23",
		9999 * time.Millisecond:   "00:10.00",
		754321 * time.Millisecond: "12:34.32",
	}

	for d, want := range cases {
		if got := stamp(d); got != want {
			t.Errorf("stamp(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestListenTakesALanguageAndAFileToHearInstead(t *testing.T) {
	cases := []struct {
		args []string
		want hearing
	}{
		{nil, hearing{}},
		{[]string{"a.wav"}, hearing{file: "a.wav"}},
		{[]string{"-lang", "fr"}, hearing{language: "fr"}},
		{[]string{"-lang=en", "a.wav"}, hearing{language: "en", file: "a.wav"}},
		{[]string{"-lang", "auto"}, hearing{language: "auto"}},
	}

	for _, c := range cases {
		got, err := parseListen(c.args)
		if err != nil || got != c.want {
			t.Errorf("%q gave %+v, %v; want %+v", c.args, got, err, c.want)
		}
	}
}

func TestTheLanguageAskedForWinsOverTheSettings(t *testing.T) {
	s := settings.Defaults()
	s.Language = "en"

	if got := (hearing{language: "fr"}).over(s); got.Language != "fr" {
		t.Errorf("asked for fr over en, got %s", got.Language)
	}

	if got := (hearing{}).over(s); got != s {
		t.Errorf("asked for nothing, got %+v, want the settings %+v", got, s)
	}
}

func TestSettleLaysTheAskedLanguageOverTheSettings(t *testing.T) {
	inEnglish := offline
	inEnglish.settings = func() (settings.Settings, error) {
		s := settings.Defaults()
		s.Language = "en"

		return s, nil
	}

	if _, s, err := settle([]string{"-lang", "fr"}, inEnglish); err != nil || s.Language != "fr" {
		t.Errorf("asked for fr over en, got %q, %v", s.Language, err)
	}

	if _, s, err := settle(nil, inEnglish); err != nil || s.Language != "en" {
		t.Errorf("asked for nothing, got %q, %v; want the settings' en", s.Language, err)
	}

	if _, _, err := settle([]string{"-lang", "de"}, inEnglish); !errors.Is(err, errUsage) {
		t.Errorf("asked for de, got %v, want the usage", err)
	}
}

func TestEndpointNamesTheServerPathAtItsAddress(t *testing.T) {
	cases := []struct {
		address, path, want string
	}{
		{"127.0.0.1:8178", "/inference", "http://127.0.0.1:8178/inference"},
		{"[::1]:5000", "/translate", "http://[::1]:5000/translate"},
		{"gpu-box.local:8178", "/inference", "http://gpu-box.local:8178/inference"},
	}

	for _, c := range cases {
		if got := endpoint(settings.Server{Address: c.address}, c.path); got != c.want {
			t.Errorf("endpoint(%s, %s) = %s, want %s", c.address, c.path, got, c.want)
		}
	}
}

func TestDescribePutsTheTranslationUnderTheTextOfAPhrase(t *testing.T) {
	phrase := vad.Phrase{Start: 1800 * time.Millisecond, End: 4400 * time.Millisecond}
	took := pipeline.Timing{Recognize: 412 * time.Millisecond, Translate: 127 * time.Millisecond}
	cases := []struct {
		result pipeline.Result
		want   string
	}{
		{
			pipeline.Result{Phrase: phrase, Text: "Bonjour à tous.", Translation: "Всем здравствуйте.", Took: took},
			"00:01.80  00:04.40   2.60 s  (0.41 s)  Bonjour à tous.\n" +
				"                             (0.13 s)  Всем здравствуйте.",
		},
		{
			pipeline.Result{Phrase: phrase, Text: "Bonjour à tous.", Took: took, Err: translate.ErrNoAnswer},
			"00:01.80  00:04.40   2.60 s  (0.41 s)  Bonjour à tous.\n" +
				"                             " + translate.ErrNoAnswer.Error(),
		},
		{pipeline.Result{Phrase: phrase, Took: pipeline.Timing{Recognize: 300 * time.Millisecond}}, "00:01.80  00:04.40   2.60 s  (0.30 s)"},
		{pipeline.Result{Phrase: phrase, Err: pipeline.ErrSkipped}, "00:01.80  00:04.40   2.60 s  " + pipeline.ErrSkipped.Error()},
		{pipeline.Result{Phrase: phrase, Err: stt.ErrNoAnswer}, "00:01.80  00:04.40   2.60 s  " + stt.ErrNoAnswer.Error()},
	}

	for _, c := range cases {
		if got := describe(c.result); got != c.want {
			t.Errorf("got\n%s\nwant\n%s", got, c.want)
		}
	}
}

func TestARecordingHandsOutAllItsSamplesOnceAndThenEnds(t *testing.T) {
	r := &recording{samples: []float32{1, 2, 3}}

	samples, err := r.Read()
	if err != nil || !slices.Equal(samples, []float32{1, 2, 3}) {
		t.Fatalf("first read gave %v, %v; want 1 2 3", samples, err)
	}

	if samples, err := r.Read(); !errors.Is(err, capture.ErrClosed) {
		t.Errorf("second read gave %v, %v; want the end", samples, err)
	}
}
