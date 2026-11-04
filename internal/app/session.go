package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/controls"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/translate"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

type Live struct {
	Language string
	Target   string
	Look     ui.Look
	Shown    int           // pairs on screen at once
	Linger   time.Duration // how long a pair stays; zero keeps it until newer ones push it out
}

type Config struct {
	Live
	Window    ui.Window
	Panel     controls.Panel // nil when there is no tray icon
	Languages []string
	Remember  func(language string) error // keeps a chosen language for the next start; nil keeps none
	Place     func(ui.Point) error        // keeps where the subtitles were moved to; nil keeps nothing
	MoveHint  string
	Now       func() time.Time             // nil reads the clock
	Configure func(adopt func(Live)) error // opens the settings, which hand back through adopt what changes at once; nil opens none
}

type heard struct {
	pair ui.Pair
	at   time.Time
}

type Session struct {
	config    Config
	ctx       context.Context
	quit      context.CancelFunc
	mu        sync.Mutex
	pairs     []heard
	state     ui.State
	listening bool
	paused    bool
	hidden    bool
	moving    bool
	live      Live
	starting  string
	told      controls.Status
	err       error
	closed    bool
}

func NewSession(c Config) *Session {
	ctx, quit := context.WithCancel(context.Background())
	s := &Session{config: c, ctx: ctx, quit: quit, live: c.Live}

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
		s.pairs = append(s.pairs, heard{pair: ui.Pair{Original: r.Text, Translation: r.Translation}, at: s.now()})
		s.pairs = s.pairs[max(0, len(s.pairs)-s.live.Shown):]
	}

	s.state = ui.Listening
	if r.Err != nil {
		s.state = ui.Failed
	}

	s.show()
}

func (s *Session) Expire() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.live.Linger == 0 {
		return
	}

	now := s.now()
	kept := make([]heard, 0, len(s.pairs))
	for _, h := range s.pairs {
		if now.Sub(h.at) < s.live.Linger {
			kept = append(kept, h)
		}
	}

	if len(kept) == len(s.pairs) {
		return
	}

	s.pairs = kept
	s.show()
}

func (s *Session) now() time.Time {
	if s.config.Now == nil {
		return time.Now()
	}

	return s.config.Now()
}

func (s *Session) Language() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.live.Language
}

func (s *Session) languages() (source, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.live.Language, s.live.Target
}

func (s *Session) Adopt(l Live) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil || s.closed {
		return
	}

	if !slices.Contains(s.config.Languages, l.Language) {
		l.Language = s.live.Language
	}
	s.live = l
	s.pairs = s.pairs[max(0, len(s.pairs)-l.Shown):]

	if err := s.config.Window.Style(l.Look); err != nil {
		s.fail(err)

		return
	}
	s.show()
}

func (s *Session) Choose(c controls.Choice) {
	switch c.Command {
	case controls.TogglePause:
		s.toggle(&s.paused)
	case controls.ToggleHidden:
		s.toggle(&s.hidden)
	case controls.ChooseLanguage:
		s.speak(c.Language)
	case controls.CopyLast:
		s.copyLast()
	case controls.ToggleMove:
		s.toggleMove()
	case controls.OpenSettings:
		s.configure()
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

func (s *Session) Translator(from func(source, target string) translate.Translator) translate.Translator {
	return translator(func(ctx context.Context, text string) (string, error) {
		return from(s.languages()).Translate(ctx, text)
	})
}

func (s *Session) Close() error {
	s.Quit()

	s.mu.Lock()
	s.closed = true
	failure := s.err
	s.mu.Unlock()

	closed := s.config.Window.Close()
	if s.config.Panel != nil {
		closed = errors.Join(closed, s.config.Panel.Close())
	}

	if failure != nil {
		return failure
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
	s.live.Language = language
	s.show()
	s.mu.Unlock()

	if s.config.Remember == nil {
		return
	}

	if err := s.config.Remember(language); err != nil {
		s.alert(err)
	}
}

func (s *Session) toggleMove() {
	s.mu.Lock()
	s.moving = !s.moving
	moving := s.moving
	s.show()
	s.mu.Unlock()

	if moving || s.config.Place == nil {
		return
	}

	if err := s.config.Place(s.config.Window.Corner()); err != nil {
		s.alert(err)
	}
}

func (s *Session) configure() {
	if s.config.Configure == nil {
		return
	}

	if err := s.config.Configure(s.Adopt); err != nil {
		s.alert(err)
	}
}

func (s *Session) copyLast() {
	s.mu.Lock()
	if len(s.pairs) == 0 || s.config.Panel == nil {
		s.mu.Unlock()

		return
	}

	last := s.pairs[len(s.pairs)-1].pair
	s.mu.Unlock()

	text := last.Original
	if last.Translation != "" {
		text += "\n" + last.Translation
	}

	if err := s.config.Panel.Copy(text); err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.fail(err)
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
	if s.err != nil || s.closed {
		return
	}

	state := s.state
	if s.paused || !s.listening {
		state = ui.Paused
	}

	pairs := make([]ui.Pair, 0, len(s.pairs)+1)
	for _, h := range s.pairs {
		pairs = append(pairs, h.pair)
	}

	if s.moving {
		pairs = append(pairs, ui.Pair{Original: s.config.MoveHint})
	}

	if err := s.config.Window.Show(ui.Frame{Pairs: pairs, State: state, Hidden: s.hidden && !s.moving, Moving: s.moving}); err != nil {
		s.fail(err)

		return
	}

	status := controls.Status{State: state, Language: s.live.Language, Target: s.live.Target, Paused: s.paused, Hidden: s.hidden, Moving: s.moving, Starting: s.starting}
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
