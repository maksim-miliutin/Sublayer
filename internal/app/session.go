package app

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/controls"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/translate"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

type Config struct {
	Window    ui.Window
	Panel     controls.Panel // nil when there is no tray icon
	Shown     int            // pairs on screen at once
	Language  string
	Languages []string
	Target    string
	Remember  func(language string) error // keeps a chosen language for the next start; nil keeps none
}

type Session struct {
	config    Config
	ctx       context.Context
	quit      context.CancelFunc
	mu        sync.Mutex
	pairs     []ui.Pair
	state     ui.State
	listening bool
	paused    bool
	hidden    bool
	language  string
	starting  string
	told      controls.Status
	err       error
}

func NewSession(c Config) *Session {
	ctx, quit := context.WithCancel(context.Background())
	s := &Session{config: c, ctx: ctx, quit: quit, language: c.Language}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.show()

	return s
}

func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) Quit() {
	s.quit()
}

func (s *Session) Starting(server string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.starting = server
	s.show()
}

func (s *Session) Ready() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listening = true
	s.starting = ""
	s.state = ui.Listening
	s.show()
}

func (s *Session) Recognizing(vad.Phrase) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = ui.Recognizing
	s.show()
}

func (s *Session) Report(r pipeline.Result) {
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

func (s *Session) Language() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.language
}

func (s *Session) Choose(c controls.Choice) {
	switch c.Command {
	case controls.TogglePause:
		s.toggle(&s.paused)
	case controls.ToggleHidden:
		s.toggle(&s.hidden)
	case controls.ChooseLanguage:
		s.speak(c.Language)
	case controls.Quit:
		s.Quit()
	}
}

func (s *Session) Follow(choices <-chan controls.Choice) {
	for c := range choices {
		s.Choose(c)
	}
}

func (s *Session) Listen(source capture.Source) capture.Source {
	return gate{session: s, source: source}
}

func (s *Session) Transcriber(in func(language string) stt.Transcriber) stt.Transcriber {
	return transcriber(func(ctx context.Context, speech []float32, rate int) (string, error) {
		return in(s.Language()).Transcribe(ctx, speech, rate)
	})
}

func (s *Session) Translator(from func(language string) translate.Translator) translate.Translator {
	return translator(func(ctx context.Context, text string) (string, error) {
		return from(s.Language()).Translate(ctx, text)
	})
}

func (s *Session) Close() error {
	s.Quit()
	closed := s.config.Window.Close()
	if s.config.Panel != nil {
		closed = errors.Join(closed, s.config.Panel.Close())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}

	return closed
}

func (s *Session) toggle(flag *bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	*flag = !*flag
	s.show()
}

func (s *Session) speak(language string) {
	if !slices.Contains(s.config.Languages, language) {
		return
	}

	s.mu.Lock()
	s.language = language
	s.show()
	s.mu.Unlock()

	if s.config.Remember == nil {
		return
	}

	if err := s.config.Remember(language); err != nil {
		s.alert(err)
	}
}

func (s *Session) alert(failure error) {
	if s.config.Panel == nil {
		return
	}

	err := s.config.Panel.Alert(failure.Error())
	if err == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail(err)
}

func (s *Session) held() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.paused
}

func (s *Session) show() {
	if s.err != nil {
		return
	}

	state := s.state
	if s.paused || !s.listening {
		state = ui.Paused
	}

	if err := s.config.Window.Show(ui.Frame{Pairs: s.pairs, State: state, Hidden: s.hidden}); err != nil {
		s.fail(err)

		return
	}

	status := controls.Status{State: state, Language: s.language, Target: s.config.Target, Paused: s.paused, Hidden: s.hidden, Starting: s.starting}
	if status == s.told || s.config.Panel == nil {
		return
	}

	if err := s.config.Panel.Show(status); err != nil {
		s.fail(err)

		return
	}
	s.told = status
}

func (s *Session) fail(err error) {
	if s.err == nil {
		s.err = err
	}
	s.quit()
}

type gate struct {
	session *Session
	source  capture.Source
}

func (g gate) Read() ([]float32, error) {
	chunk, err := g.source.Read()
	if err != nil || !g.session.held() {
		return chunk, err
	}

	// Silence ends a phrase in progress and starts no other, while the device is still read.
	return make([]float32, len(chunk)), nil
}

func (g gate) Close() error {
	return g.source.Close()
}

type transcriber func(ctx context.Context, speech []float32, rate int) (string, error)

func (t transcriber) Transcribe(ctx context.Context, speech []float32, rate int) (string, error) {
	return t(ctx, speech, rate)
}

type translator func(ctx context.Context, text string) (string, error)

func (t translator) Translate(ctx context.Context, text string) (string, error) {
	return t(ctx, text)
}
