package app

import (
	"errors"
	"sync"

	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

type Config struct {
	Window        ui.Window
	Shown         int // pairs on screen at once
	StopListening func()
}

type Subtitles struct {
	config Config
	mu     sync.Mutex
	pairs  []ui.Pair
	state  ui.State
	err    error
}

func NewSubtitles(c Config) *Subtitles {
	return &Subtitles{config: c}
}

func (s *Subtitles) Recognizing(vad.Phrase) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = ui.Recognizing
	s.show()
}

func (s *Subtitles) Report(r pipeline.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if errors.Is(r.Err, pipeline.ErrSkipped) {
		return
	}

	if r.Text != "" {
		s.pairs = append(s.pairs, ui.Pair{Original: r.Text, Translation: r.Translation})
		s.pairs = s.pairs[max(0, len(s.pairs)-s.config.Shown):]
	}

	s.state = ui.Listening
	if r.Err != nil {
		s.state = ui.Failed
	}

	s.show()
}

func (s *Subtitles) Close() error {
	closed := s.config.Window.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}

	return closed
}

func (s *Subtitles) show() {
	if s.err != nil {
		return
	}

	err := s.config.Window.Show(ui.Frame{Pairs: s.pairs, State: s.state})
	if err == nil {
		return
	}

	s.err = err
	s.config.StopListening()
}
