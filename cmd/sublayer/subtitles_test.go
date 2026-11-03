package main

import (
	"sync"
	"testing"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/app"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

type screen struct {
	mu     sync.Mutex
	frames []ui.Frame
}

func (s *screen) Show(f ui.Frame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frames = append(s.frames, f)

	return nil
}

func (s *screen) Corner() ui.Point {
	return ui.Point{}
}

func (s *screen) Close() error {
	return nil
}

func (s *screen) pairs() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.frames[len(s.frames)-1].Pairs)
}

func TestLookOfCarriesTheOverlaySettings(t *testing.T) {
	s := settings.Defaults()
	s.Overlay = settings.Overlay{TextSize: 14, WidthPercent: 33, BackgroundPercent: 50, LingerSeconds: 4}

	if got := lookOf(s); got != (ui.Look{TextPoints: 14, WidthPercent: 33, BackgroundPercent: 50}) {
		t.Errorf("got %+v", got)
	}
}

func TestExpireLetsPairsGoOnEachTickUntilTheSessionEnds(t *testing.T) {
	w, at := &screen{}, time.Unix(1000, 0)
	session := app.NewSession(app.Config{Window: w, Shown: 3, Linger: time.Second, Now: func() time.Time { return at }})
	session.Ready()
	session.Report(pipeline.Result{Text: "Oui.", Translation: "Да."})
	at = at.Add(2 * time.Second)

	ticks, done := make(chan time.Time), make(chan struct{})
	go func() {
		expire(session, ticks)
		close(done)
	}()

	ticks <- time.Time{}
	session.Quit()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expire still runs after the session ended")
	}

	if got := w.pairs(); got != 0 {
		t.Errorf("%d pairs still show past their time, want none", got)
	}
}

func TestTheSettingsKeepWhatTheSessionChose(t *testing.T) {
	s := speaking("en")(settings.Defaults())
	s = placed(ui.Point{X: 40, Y: 980})(s)

	if s.Language != "en" || s.Corner == nil || *s.Corner != (settings.Corner{X: 40, Y: 980}) {
		t.Errorf("got %q and %+v, want en and the corner at 40, 980", s.Language, s.Corner)
	}

	if got := cornerOf(s); got == nil || *got != (ui.Point{X: 40, Y: 980}) {
		t.Errorf("the window would open at %+v, want 40, 980", got)
	}

	if got := cornerOf(settings.Defaults()); got != nil {
		t.Errorf("without a corner the window would open at %+v, want its default", got)
	}
}
