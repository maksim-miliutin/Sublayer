package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

func TestDefaultsSurviveAWriteAndARead(t *testing.T) {
	file, err := Encode(Defaults())
	if err != nil {
		t.Fatal(err)
	}

	if got, err := Parse(file); err != nil || got != Defaults() {
		t.Errorf("read back %+v, %v; want the defaults %+v", got, err, Defaults())
	}
}

func TestDefaultsCutPhrasesTheWayTheCutterDoes(t *testing.T) {
	if got := Defaults().Phrases.Config(); got != vad.Defaults() {
		t.Errorf("the default phrases cut as %+v, want the cutter's own %+v", got, vad.Defaults())
	}
}

func TestPhrasesTurnMillisecondsIntoTheCuttersConfig(t *testing.T) {
	p := Phrases{MinSpeechMS: 250, EndPauseMS: 800, MaxPhraseMS: 9000}
	want := vad.Config{MinSpeech: 250 * time.Millisecond, EndPause: 800 * time.Millisecond, MaxPhrase: 9 * time.Second}

	if got := p.Config(); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseKeepsTheDefaultsForWhatTheFileLeavesOut(t *testing.T) {
	want := Defaults()
	want.Language = "fr"
	want.Phrases.EndPauseMS = 800

	got, err := Parse([]byte(`{"language": "fr", "phrases": {"end_pause_ms": 800}}`))
	if err != nil || got != want {
		t.Errorf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestParseReadsAFileThatStartsWithAByteOrderMark(t *testing.T) {
	got, err := Parse([]byte("\xef\xbb\xbf{\"pairs\": 5}"))
	if err != nil || got.Pairs != 5 {
		t.Errorf("got %d pairs, %v; want 5", got.Pairs, err)
	}
}

func TestParseRefusesSettingsItCannotUse(t *testing.T) {
	refused := map[string]string{
		"broken json":                     `{"language": "fr"`,
		"empty file":                      ``,
		"two documents":                   `{} {}`,
		"misspelt name":                   `{"langauge": "fr"}`,
		"wrong type":                      `{"pairs": "four"}`,
		"unknown language":                `{"language": "de"}`,
		"unknown target":                  `{"target": "fr"}`,
		"too few pairs":                   `{"pairs": 2}`,
		"too many pairs":                  `{"pairs": 7}`,
		"address without a port":          `{"whisper": {"address": "127.0.0.1"}}`,
		"address without a host":          `{"libretranslate": {"address": ":5000"}}`,
		"port named, not numbered":        `{"libretranslate": {"address": "localhost:http"}}`,
		"port zero":                       `{"whisper": {"address": "127.0.0.1:0"}}`,
		"port past the last":              `{"whisper": {"address": "127.0.0.1:65536"}}`,
		"no speech needed":                `{"phrases": {"min_speech_ms": 0}}`,
		"no pause needed":                 `{"phrases": {"end_pause_ms": 0}}`,
		"pause below zero":                `{"phrases": {"end_pause_ms": -1}}`,
		"no phrase at all":                `{"phrases": {"max_phrase_ms": 0}}`,
		"whisper started without a model": `{"whisper": {"model": ""}}`,
		"a key without a modifier":        `{"keys": {"show": "T"}}`,
		"one key for two things":          `{"keys": {"pause": "Ctrl+Alt+T"}}`,
		"a copy key without a modifier":   `{"keys": {"copy": "C"}}`,
		"a move key without a modifier":   `{"keys": {"move": "M"}}`,
		"speech as long as a phrase":      `{"phrases": {"min_speech_ms": 12000}}`,
		"pause as long as a phrase":       `{"phrases": {"end_pause_ms": 12000}}`,
	}

	for name, file := range refused {
		if _, err := Parse([]byte(file)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
}

func TestParseTakesTheEdgesOfWhatItAllows(t *testing.T) {
	allowed := []string{
		`{"pairs": 3}`,
		`{"pairs": 6}`,
		`{"target": "en", "language": "fr"}`,
		`{"whisper": {"address": "localhost:1"}, "libretranslate": {"address": "[::1]:65535"}}`,
		`{"phrases": {"min_speech_ms": 1, "end_pause_ms": 1, "max_phrase_ms": 2}}`,
		`{"whisper": {"program": "", "model": ""}}`,
		`{"libretranslate": {"program": "C:/lt-venv/Scripts/libretranslate.exe"}}`,
		`{"keys": {"show": "Ctrl+Shift+F9", "copy": "win+alt+c"}}`,
	}

	for _, file := range allowed {
		if _, err := Parse([]byte(file)); err != nil {
			t.Errorf("%s: got %v, want it taken", file, err)
		}
	}
}

func TestACornerIsKeptOnlyOnceTheSubtitlesWereMoved(t *testing.T) {
	file, err := Encode(Defaults())
	if err != nil || strings.Contains(string(file), "corner") {
		t.Errorf("the defaults wrote %v with a corner in:\n%s", err, file)
	}

	s, err := Parse([]byte(`{"corner": {"x": 120, "y": 900}}`))
	if err != nil || s.Corner == nil || *s.Corner != (Corner{X: 120, Y: 900}) {
		t.Errorf("got %+v, %v; want the corner at 120, 900", s.Corner, err)
	}
}

func TestCodesNameEachLanguageOnceAndLeaveAutoOut(t *testing.T) {
	if got := Codes(); !slices.Equal(got, []string{"fr", "en", "ru"}) {
		t.Errorf("got %q, want fr, en and ru", got)
	}
}

func TestEncodeWritesNamesAPersonCanReadAndEdit(t *testing.T) {
	file, err := Encode(Defaults())
	if err != nil {
		t.Fatal(err)
	}

	text := string(file)
	for _, line := range []string{
		`  "language": "auto",`,
		`  "target": "ru",`,
		`  "whisper": {`,
		`    "address": "127.0.0.1:8178",`,
		`    "program": "whisper-server.exe",`,
		`    "model": "models/ggml-large-v3-turbo.bin"`,
		`  "libretranslate": {`,
		`    "address": "127.0.0.1:5000",`,
		`    "program": ""`,
		`  "pairs": 4,`,
		`    "min_speech_ms": 300,`,
		`    "end_pause_ms": 600,`,
		`    "max_phrase_ms": 12000`,
		`  "keys": {`,
		`    "show": "Ctrl+Alt+T",`,
		`    "pause": "Ctrl+Alt+P",`,
		`    "copy": "Ctrl+Alt+C",`,
		`    "move": "Ctrl+Alt+M"`,
	} {
		if !strings.Contains(text, line+"\n") {
			t.Errorf("the file lacks the line %q:\n%s", line, text)
		}
	}

	if !strings.HasSuffix(text, "}\n") {
		t.Errorf("the file does not end its last line:\n%q", text)
	}
}
