package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/hotkey"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

var ErrInvalid = errors.New("settings: the file holds no settings Sublayer can use")

var Languages = []string{"auto", "fr", "en"}

var Targets = []string{"ru", "en"}

const (
	fewestPairs = 1
	mostPairs   = 6
)

type Settings struct {
	Language string  `json:"language"`
	Target   string  `json:"target"`
	Whisper  Whisper `json:"whisper"`
	Libre    Server  `json:"libretranslate"`
	Pairs    int     `json:"pairs"`
	Overlay  Overlay `json:"overlay"`
	Phrases  Phrases `json:"phrases"`
	Keys     Keys    `json:"keys"`
	Corner   *Corner `json:"corner,omitempty"` // where the subtitles were moved to; absent leaves them at the bottom left
}

type Corner struct {
	X int `json:"x"` // screen pixels of the bottom left corner of the subtitles
	Y int `json:"y"`
}

type Server struct {
	Address string `json:"address"`
	Program string `json:"program"` // started when nothing answers at the address; empty leaves that to you
}

type Whisper struct {
	Server
	Model string `json:"model"`
}

type Overlay struct {
	TextSize          int `json:"text_size"` // points of the translation; the original is three quarters of it
	WidthPercent      int `json:"width_percent"`
	BackgroundPercent int `json:"background_percent"`
	LingerSeconds     int `json:"linger_seconds"` // zero keeps a pair until newer ones push it out
}

type Keys struct {
	Show  string `json:"show"`
	Pause string `json:"pause"`
	Copy  string `json:"copy"`
	Move  string `json:"move"`
}

type Phrases struct {
	MinSpeechMS int `json:"min_speech_ms"`
	EndPauseMS  int `json:"end_pause_ms"`
	MaxPhraseMS int `json:"max_phrase_ms"`
}

func Defaults() Settings {
	cut := vad.Defaults()

	return Settings{
		Language: "auto",
		Target:   "ru",
		Whisper: Whisper{
			Server: Server{Address: "127.0.0.1:8178", Program: "whisper-server.exe"},
			Model:  "models/ggml-large-v3-turbo.bin",
		},
		Libre:   Server{Address: "127.0.0.1:5000"},
		Pairs:   3,
		Overlay: Overlay{TextSize: 12, WidthPercent: 25, BackgroundPercent: 75, LingerSeconds: 10},
		Phrases: Phrases{
			MinSpeechMS: int(cut.MinSpeech / time.Millisecond),
			EndPauseMS:  int(cut.EndPause / time.Millisecond),
			MaxPhraseMS: int(cut.MaxPhrase / time.Millisecond),
		},
		Keys: Keys{Show: "Ctrl+Alt+T", Pause: "Ctrl+Alt+P", Copy: "Ctrl+Alt+C", Move: "Ctrl+Alt+M"},
	}
}

func Codes() []string {
	var codes []string
	for _, code := range slices.Concat(Languages, Targets) {
		if code != "auto" && !slices.Contains(codes, code) {
			codes = append(codes, code)
		}
	}

	return codes
}

func (p Phrases) Config() vad.Config {
	return vad.Config{
		MinSpeech: time.Duration(p.MinSpeechMS) * time.Millisecond,
		EndPause:  time.Duration(p.EndPauseMS) * time.Millisecond,
		MaxPhrase: time.Duration(p.MaxPhraseMS) * time.Millisecond,
	}
}

func (o Overlay) Linger() time.Duration {
	return time.Duration(o.LingerSeconds) * time.Second
}

func Parse(file []byte) (Settings, error) {
	// Notepad may open a UTF-8 file with a byte order mark, and JSON allows none.
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))))
	decoder.DisallowUnknownFields()

	s := Defaults()
	if err := decoder.Decode(&s); err != nil {
		return Settings{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Settings{}, fmt.Errorf("%w: more follows the settings", ErrInvalid)
	}

	if err := s.check(); err != nil {
		return Settings{}, err
	}

	return s, nil
}

func Encode(s Settings) ([]byte, error) {
	file, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(file, '\n'), nil
}

func (s Settings) check() error {
	switch {
	// whisper-server takes a language it does not know without complaint and hears nonsense,
	// so only the languages Sublayer is meant for get through.
	case !slices.Contains(Languages, s.Language):
		return fmt.Errorf("%w: language %q, want one of %s", ErrInvalid, s.Language, strings.Join(Languages, ", "))
	case !slices.Contains(Targets, s.Target):
		return fmt.Errorf("%w: target %q, want one of %s", ErrInvalid, s.Target, strings.Join(Targets, ", "))
	case s.Pairs < fewestPairs || s.Pairs > mostPairs:
		return fmt.Errorf("%w: pairs %d, want %d to %d", ErrInvalid, s.Pairs, fewestPairs, mostPairs)
	case s.Whisper.Program != "" && s.Whisper.Model == "":
		return fmt.Errorf("%w: whisper.model is empty, want the model whisper.program loads", ErrInvalid)
	}

	return errors.Join(
		checkAddress("whisper", s.Whisper.Address),
		checkAddress("libretranslate", s.Libre.Address),
		s.Phrases.check(),
		s.Overlay.check(),
		s.Keys.check(),
	)
}

func (o Overlay) check() error {
	for _, field := range []struct {
		name               string
		value, least, most int
	}{
		{"text_size", o.TextSize, 8, 40},
		{"width_percent", o.WidthPercent, 10, 80},
		{"background_percent", o.BackgroundPercent, 0, 100},
		{"linger_seconds", o.LingerSeconds, 0, 120},
	} {
		if field.value < field.least || field.value > field.most {
			return fmt.Errorf("%w: overlay.%s %d, want %d to %d", ErrInvalid, field.name, field.value, field.least, field.most)
		}
	}

	return nil
}

func (k Keys) check() error {
	held := map[hotkey.Combo]string{}
	for _, key := range []struct{ name, text string }{{"show", k.Show}, {"pause", k.Pause}, {"copy", k.Copy}, {"move", k.Move}} {
		combo, err := hotkey.Parse(key.text)
		if err != nil {
			return fmt.Errorf("%w: keys.%s: %w", ErrInvalid, key.name, err)
		}

		if other, taken := held[combo]; taken {
			return fmt.Errorf("%w: keys.%s and keys.%s are both %s", ErrInvalid, other, key.name, combo)
		}
		held[combo] = key.name
	}

	return nil
}

func checkAddress(server, address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return fmt.Errorf("%w: %s.address %q, want host:port", ErrInvalid, server, address)
	}

	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return fmt.Errorf("%w: %s.address %q, want a port from 1 to 65535", ErrInvalid, server, address)
	}

	return nil
}

func (p Phrases) check() error {
	switch {
	case p.MinSpeechMS < 1 || p.EndPauseMS < 1:
		return fmt.Errorf("%w: min_speech_ms %d and end_pause_ms %d, want both above zero", ErrInvalid, p.MinSpeechMS, p.EndPauseMS)
	case p.MinSpeechMS >= p.MaxPhraseMS || p.EndPauseMS >= p.MaxPhraseMS:
		return fmt.Errorf("%w: min_speech_ms %d and end_pause_ms %d, want both below max_phrase_ms %d", ErrInvalid, p.MinSpeechMS, p.EndPauseMS, p.MaxPhraseMS)
	}

	return nil
}
