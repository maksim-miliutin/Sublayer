package main

import (
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/app"
	"github.com/maksim-miliutin/Sublayer/internal/controls"
	"github.com/maksim-miliutin/Sublayer/internal/form"
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

type store struct {
	file   settings.Settings
	broken error
}

func (s *store) machine() machine {
	return machine{
		settings: func() (settings.Settings, error) { return s.file, s.broken },
		update: func(change func(settings.Settings) settings.Settings) error {
			s.file = change(s.file)

			return nil
		},
	}
}

type editing struct {
	config form.Config
	opened int
}

func (e *editing) Edit(c form.Config) error {
	e.config, e.opened = c, e.opened+1

	return nil
}

func labelled(t *testing.T, sections []form.Section, label string) *form.Field {
	t.Helper()

	var all []*form.Field
	for i := range sections {
		for j := range sections[i].Fields {
			all = append(all, &sections[i].Fields[j])
		}
	}

	at := slices.IndexFunc(all, func(f *form.Field) bool { return f.Label == label })
	if at < 0 {
		t.Fatalf("no field is labelled %q", label)
	}

	return all[at]
}

func opened(t *testing.T, file *store, choose func(controls.Choice)) (*editing, []form.Section) {
	t.Helper()

	window := &editing{}
	if err := configure(file.machine(), window, choose); err != nil {
		t.Fatal(err)
	}

	edited, err := form.Sections(window.config.Settings)
	if err != nil {
		t.Fatal(err)
	}

	return window, edited
}

func TestTheSettingsWindowSavesOverWhatTheFileHoldsByThen(t *testing.T) {
	file := &store{file: settings.Defaults()}
	var chosen []controls.Choice
	window, edited := opened(t, file, func(c controls.Choice) { chosen = append(chosen, c) })

	labelled(t, edited, "Text size, pt").Value = "16"
	labelled(t, edited, "The other side speaks").Value = "en"
	file.file.Corner = &settings.Corner{X: 40, Y: 980}

	if err := window.config.Save(edited); err != nil {
		t.Fatal(err)
	}

	got := file.file
	if got.Overlay.TextSize != 16 || got.Language != "en" || got.Corner == nil || *got.Corner != (settings.Corner{X: 40, Y: 980}) {
		t.Errorf("the file holds text size %d, %q and corner %+v; want 16, en and the corner kept", got.Overlay.TextSize, got.Language, got.Corner)
	}

	if want := []controls.Choice{{Command: controls.ChooseLanguage, Language: "en"}}; !slices.Equal(chosen, want) {
		t.Errorf("chose %+v, want %+v", chosen, want)
	}
}

func TestAWrongFieldLeavesTheFileAsItWas(t *testing.T) {
	file := &store{file: settings.Defaults()}
	chosen := 0
	window, edited := opened(t, file, func(controls.Choice) { chosen++ })

	labelled(t, edited, "Text size, pt").Value = "7"
	err := window.config.Save(edited)

	if !errors.Is(err, settings.ErrInvalid) || !reflect.DeepEqual(file.file, settings.Defaults()) || chosen != 0 {
		t.Errorf("got %v with %d choices and the file %+v; want settings.ErrInvalid, no choice and the file untouched", err, chosen, file.file)
	}
}

func TestSettingsSublayerCannotReadOpenNoWindow(t *testing.T) {
	window := &editing{}
	err := configure((&store{broken: settings.ErrInvalid}).machine(), window, func(controls.Choice) {})

	if !errors.Is(err, settings.ErrInvalid) || window.opened != 0 {
		t.Errorf("got %v and %d windows, want settings.ErrInvalid and none", err, window.opened)
	}
}
