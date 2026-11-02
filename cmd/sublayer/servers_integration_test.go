//go:build integration

package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
	"github.com/maksim-miliutin/Sublayer/internal/translate"
	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

func russian(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(unicode.Cyrillic, r) })
}

func TestLibreTranslateTurnsTheSpeechLanguagesIntoRussian(t *testing.T) {
	said := map[string]string{
		"fr":   "Bonjour, merci d'être là aujourd'hui.",
		"en":   "The deadline is next Friday.",
		"auto": "Je vous rappelle demain matin.",
	}

	s := settings.Defaults()
	for language, text := range said {
		l := translate.NewLibre(translate.Config{URL: endpoint(s.Libre, librePath), Source: language, Target: s.Target}, localClient())

		got, err := l.Translate(context.Background(), text)
		if err != nil || !russian(got) {
			t.Errorf("%s %q came back as %q, %v; want it in Russian", language, text, got, err)
		}

		t.Logf("%s: %s -> %s", language, text, got)
	}
}

func TestTheServersHearAndTranslateTheRecordedSpeech(t *testing.T) {
	path := os.Getenv("SUBLAYER_SPEECH")
	if path == "" {
		t.Skip("SUBLAYER_SPEECH names no wav file of speech for the servers to hear")
	}

	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	samples, rate, err := wav.Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	var heard []pipeline.Result
	report := func(r pipeline.Result) {
		t.Log(describe(r))
		heard = append(heard, r)
	}

	if err := interpret(pipeline.Config{Source: &recording{samples: samples}, Rate: rate, Report: report}, settings.Defaults()); err != nil {
		t.Fatal(err)
	}

	words := 0
	for _, r := range heard {
		if r.Err != nil {
			t.Errorf("the phrase at %s came back with %v", stamp(r.Phrase.Start), r.Err)
		}

		if r.Text != "" && !russian(r.Translation) {
			t.Errorf("the phrase at %s, %q, was translated as %q, want it in Russian", stamp(r.Phrase.Start), r.Text, r.Translation)
		}

		words += len(strings.Fields(r.Text))
	}

	if words == 0 {
		t.Errorf("the servers heard no words in the %d phrases of %s", len(heard), path)
	}
}
