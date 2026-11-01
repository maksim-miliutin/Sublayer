//go:build integration

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

func TestWhisperServerHearsTheRecordedSpeech(t *testing.T) {
	path := os.Getenv("SUBLAYER_SPEECH")
	if path == "" {
		t.Skip("SUBLAYER_SPEECH names no wav file of speech for whisper-server to hear")
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

	if err := recognize(pipeline.Config{Source: &recording{samples: samples}, Rate: rate, Report: report}, "auto"); err != nil {
		t.Fatal(err)
	}

	words := 0
	for _, r := range heard {
		if r.Err != nil {
			t.Errorf("the phrase at %s was not recognized: %v", stamp(r.Phrase.Start), r.Err)
		}

		words += len(strings.Fields(r.Text))
	}

	if words == 0 {
		t.Errorf("whisper-server heard no words in the %d phrases of %s", len(heard), path)
	}
}
