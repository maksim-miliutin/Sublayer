package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/controls"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/translate"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

var (
	errBroken   = errors.New("ui: the overlay window failed")
	errReadOnly = errors.New("settings: the file is read only")
)

type window struct {
	corner    ui.Point
	frames    []ui.Frame
	looks     []ui.Look
	fail      error
	styleFail error
	closeErr  error
	closed    bool
}

func (w *window) Show(f ui.Frame) error {
	if w.fail != nil {
		return w.fail
	}

	w.frames = append(w.frames, f)

	return nil
}

func (w *window) Style(l ui.Look) error {
	if w.styleFail != nil {
		return w.styleFail
	}

	w.looks = append(w.looks, l)

	return nil
}

func (w *window) Corner() ui.Point {
	return w.corner
}

func (w *window) Close() error {
	w.closed = true

	return w.closeErr
}

func (w *window) last(t *testing.T) ui.Frame {
	t.Helper()

	if len(w.frames) == 0 {
		t.Fatal("the window was never shown a frame")
	}

	return w.frames[len(w.frames)-1]
}

type panel struct {
	statuses  []controls.Status
	alerts    []string
	copied    []string
	fail      error
	alertFail error
	copyFail  error
	closed    bool
}

func (p *panel) Show(s controls.Status) error {
	if p.fail != nil {
		return p.fail
	}

	p.statuses = append(p.statuses, s)

	return nil
}

func (p *panel) Alert(text string) error {
	if p.alertFail != nil {
		return p.alertFail
	}

	p.alerts = append(p.alerts, text)

	return nil
}

func (p *panel) Copy(text string) error {
	if p.copyFail != nil {
		return p.copyFail
	}

	p.copied = append(p.copied, text)

	return nil
}

func (p *panel) Choices() <-chan controls.Choice {
	return nil
}

func (p *panel) Close() error {
	p.closed = true

	return nil
}

func (p *panel) last(t *testing.T) controls.Status {
	t.Helper()

	if len(p.statuses) == 0 {
		t.Fatal("the panel was never shown a status")
	}

	return p.statuses[len(p.statuses)-1]
}

type speakers struct {
	chunks [][]float32
}

func (s *speakers) Read() ([]float32, error) {
	chunk := s.chunks[0]
	s.chunks = s.chunks[1:]

	return chunk, nil
}

func (s *speakers) Close() error {
	return nil
}

func config(w *window, p *panel, shown int) Config {
	return Config{Live: Live{Language: "fr", Target: "ru", Shown: shown}, Window: w, Panel: p, Languages: []string{"auto", "fr", "en"}, MoveHint: "Drag me"}
}

func listening(w *window, shown int) *Session {
	s := NewSession(config(w, &panel{}, shown))
	s.Ready()

	return s
}

func said(text, translation string) pipeline.Result {
	return pipeline.Result{Text: text, Translation: translation}
}

func originals(f ui.Frame) []string {
	var texts []string
	for _, pair := range f.Pairs {
		texts = append(texts, pair.Original)
	}

	return texts
}

func TestSessionPutsEachNewPairUnderTheOnesBefore(t *testing.T) {
	w := &window{}
	s := listening(w, 4)

	s.Report(said("Bonjour.", "Здравствуйте."))
	s.Report(said("Merci.", "Спасибо."))

	want := []ui.Pair{{Original: "Bonjour.", Translation: "Здравствуйте."}, {Original: "Merci.", Translation: "Спасибо."}}
	if got := w.last(t); !slices.Equal(got.Pairs, want) || got.State != ui.Listening {
		t.Errorf("got %v in state %d, want %v while listening", got.Pairs, got.State, want)
	}
}

func TestSessionKeepsOnlyAsManyPairsAsAreShown(t *testing.T) {
	w := &window{}
	s := listening(w, 2)

	for _, text := range []string{"Un.", "Deux.", "Trois."} {
		s.Report(said(text, "?"))
	}

	if got := w.last(t).Pairs; len(got) != 2 || got[0].Original != "Deux." || got[1].Original != "Trois." {
		t.Errorf("got %v, want the latest two, Deux. then Trois.", got)
	}
}

func TestSessionLightsTheDotWhileAPhraseIsRecognized(t *testing.T) {
	w := &window{}
	s := listening(w, 4)

	s.Recognizing(vad.Phrase{})
	if got := w.last(t).State; got != ui.Recognizing {
		t.Errorf("a phrase in recognition shows state %d, want recognizing", got)
	}

	s.Report(said("Oui.", "Да."))
	if got := w.last(t).State; got != ui.Listening {
		t.Errorf("a recognized phrase leaves state %d, want listening again", got)
	}
}

func TestSessionShowsAFailureUntilAPhraseComesThrough(t *testing.T) {
	w := &window{}
	s := listening(w, 4)

	s.Report(pipeline.Result{Err: stt.ErrNoAnswer})
	if got := w.last(t); got.State != ui.Failed || len(got.Pairs) != 0 {
		t.Errorf("a failed recognition shows %v in state %d, want no pair and the failure", got.Pairs, got.State)
	}

	s.Recognizing(vad.Phrase{})
	s.Report(said("Oui.", "Да."))
	if got := w.last(t).State; got != ui.Listening {
		t.Errorf("a phrase that came through leaves state %d, want listening", got)
	}
}

func TestSessionShowsTheOriginalOfAPhraseLeftUntranslated(t *testing.T) {
	w := &window{}
	s := listening(w, 4)

	s.Report(pipeline.Result{Text: "Oui.", Err: translate.ErrNoAnswer})

	want := []ui.Pair{{Original: "Oui."}}
	if got := w.last(t); !slices.Equal(got.Pairs, want) || got.State != ui.Failed {
		t.Errorf("got %v in state %d, want the original alone and the failure", got.Pairs, got.State)
	}
}

func TestSessionLeavesOutAPhraseWithoutWords(t *testing.T) {
	w := &window{}
	s := listening(w, 4)

	s.Report(said("Oui.", "Да."))
	s.Recognizing(vad.Phrase{})
	s.Report(pipeline.Result{})

	if got := w.last(t); len(got.Pairs) != 1 || got.State != ui.Listening {
		t.Errorf("got %v in state %d, want the one pair and listening", got.Pairs, got.State)
	}
}

func TestSessionLeavesTheDotAloneForASkippedPhrase(t *testing.T) {
	w := &window{}
	s := listening(w, 4)

	s.Recognizing(vad.Phrase{})
	s.Report(pipeline.Result{Err: pipeline.ErrSkipped})

	if got := w.last(t); got.State != ui.Recognizing || len(got.Pairs) != 0 {
		t.Errorf("a skipped phrase left %v in state %d, want recognition still showing", got.Pairs, got.State)
	}
}

func TestSessionStaysGreyAndTellsWhatStartsUntilItListens(t *testing.T) {
	w, p := &window{}, &panel{}
	s := NewSession(config(w, p, 4))

	if got := w.last(t).State; got != ui.Paused {
		t.Errorf("a session not yet listening shows state %d, want the grey of a pause", got)
	}

	s.Starting("whisper-server")
	if got := p.last(t); got.Starting != "whisper-server" || got.State != ui.Paused {
		t.Errorf("the panel shows %+v, want whisper-server starting and the grey dot", got)
	}

	s.Ready()
	if got := p.last(t); got.Starting != "" || got.State != ui.Listening || w.last(t).State != ui.Listening {
		t.Errorf("the panel shows %+v and the window state %d, want both listening", got, w.last(t).State)
	}
}

func TestPauseGreysTheDotAndSilencesTheSpeakers(t *testing.T) {
	w, p := &window{}, &panel{}
	s := NewSession(config(w, p, 4))
	s.Ready()
	source := s.Listen(&speakers{chunks: [][]float32{{0.5, -0.5}, {0.25, -0.25}}})

	s.Choose(controls.Choice{Command: controls.TogglePause})
	chunk, err := source.Read()
	if err != nil || !slices.Equal(chunk, []float32{0, 0}) || w.last(t).State != ui.Paused || !p.last(t).Paused {
		t.Errorf("paused, the speakers gave %v, %v with the window in state %d; want two silent samples and the grey dot", chunk, err, w.last(t).State)
	}

	s.Choose(controls.Choice{Command: controls.TogglePause})
	chunk, err = source.Read()
	if err != nil || !slices.Equal(chunk, []float32{0.25, -0.25}) || w.last(t).State != ui.Listening || p.last(t).Paused {
		t.Errorf("resumed, the speakers gave %v, %v with the window in state %d; want the sound and listening", chunk, err, w.last(t).State)
	}
}

func TestPauseKeepsTheDotGreyWhileAHeardPhraseFinishes(t *testing.T) {
	w := &window{}
	s := listening(w, 4)

	s.Choose(controls.Choice{Command: controls.TogglePause})
	s.Recognizing(vad.Phrase{})
	s.Report(said("Oui.", "Да."))

	if got := w.last(t); got.State != ui.Paused || len(got.Pairs) != 1 {
		t.Errorf("got %v in state %d, want the finished pair under the grey dot", got.Pairs, got.State)
	}
}

func TestHidingTakesTheSubtitlesAwayButKeepsTheirPairs(t *testing.T) {
	w, p := &window{}, &panel{}
	s := NewSession(config(w, p, 4))
	s.Ready()

	s.Choose(controls.Choice{Command: controls.ToggleHidden})
	s.Report(said("Oui.", "Да."))
	if got := w.last(t); !got.Hidden || len(got.Pairs) != 1 || !p.last(t).Hidden {
		t.Errorf("hidden, got %+v; want the pair kept out of sight", got)
	}

	s.Choose(controls.Choice{Command: controls.ToggleHidden})
	if got := w.last(t); got.Hidden || len(got.Pairs) != 1 || p.last(t).Hidden {
		t.Errorf("shown again, got %+v; want the pair in sight", got)
	}
}

func TestAChosenLanguageGoesToTheNextPhraseAndIsRemembered(t *testing.T) {
	var heard, translated, remembered []string
	p := &panel{}
	c := config(&window{}, p, 4)
	c.Remember = func(language string) error {
		remembered = append(remembered, language)

		return nil
	}
	s := NewSession(c)

	transcriber := s.Transcriber(func(language string) stt.Transcriber {
		heard = append(heard, language)

		return speech("Oui.")
	})
	translator := s.Translator(func(source, _ string) translate.Translator {
		translated = append(translated, source)

		return words("Да.")
	})

	transcriber.Transcribe(context.Background(), nil, 16000)
	translator.Translate(context.Background(), "Oui.")
	s.Choose(controls.Choice{Command: controls.ChooseLanguage, Language: "en"})
	transcriber.Transcribe(context.Background(), nil, 16000)
	translator.Translate(context.Background(), "Yes.")

	if !slices.Equal(heard, []string{"fr", "en"}) || !slices.Equal(translated, []string{"fr", "en"}) || !slices.Equal(remembered, []string{"en"}) {
		t.Errorf("heard in %q, translated from %q, remembered %q; want fr then en, and en remembered", heard, translated, remembered)
	}

	if got := s.Language(); got != "en" || p.last(t).Language != "en" {
		t.Errorf("the session speaks %q and the panel shows %q, want en in both", got, p.last(t).Language)
	}
}

func TestALanguageSublayerDoesNotKnowIsLeftUnchosen(t *testing.T) {
	remembered := 0
	c := config(&window{}, &panel{}, 4)
	c.Remember = func(string) error {
		remembered++

		return nil
	}
	s := NewSession(c)

	s.Choose(controls.Choice{Command: controls.ChooseLanguage, Language: "de"})

	if s.Language() != "fr" || remembered != 0 {
		t.Errorf("the session speaks %q with %d languages remembered, want fr and none", s.Language(), remembered)
	}
}

func TestALanguageThatCannotBeRememberedIsTold(t *testing.T) {
	p := &panel{}
	c := config(&window{}, p, 4)
	c.Remember = func(string) error { return errReadOnly }
	s := NewSession(c)

	s.Choose(controls.Choice{Command: controls.ChooseLanguage, Language: "en"})

	if s.Language() != "en" || len(p.alerts) != 1 || p.alerts[0] != errReadOnly.Error() {
		t.Errorf("the session speaks %q with alerts %q, want en and the failure told", s.Language(), p.alerts)
	}
}

func TestThePanelHearsOfAStatusOnlyWhenItChanges(t *testing.T) {
	p := &panel{}
	s := NewSession(config(&window{}, p, 4))
	s.Ready()

	s.Report(said("Un.", "Один."))
	s.Report(said("Deux.", "Два."))

	if got := len(p.statuses); got != 2 {
		t.Errorf("the panel was shown %d statuses, want 2: not listening yet, then listening", got)
	}
}

func TestCopyTakesTheLastPairOrNothingWhenThereIsNone(t *testing.T) {
	p := &panel{}
	s := NewSession(config(&window{}, p, 4))
	s.Ready()

	s.Choose(controls.Choice{Command: controls.CopyLast})
	s.Report(said("Bonjour.", "Здравствуйте."))
	s.Report(said("Merci.", "Спасибо."))
	s.Choose(controls.Choice{Command: controls.CopyLast})
	s.Report(pipeline.Result{Text: "Oui.", Err: translate.ErrNoAnswer})
	s.Choose(controls.Choice{Command: controls.CopyLast})

	if want := []string{"Merci.\nСпасибо.", "Oui."}; !slices.Equal(p.copied, want) {
		t.Errorf("copied %q, want %q", p.copied, want)
	}
}

func TestACopyThePanelCannotTakeEndsTheSession(t *testing.T) {
	s := NewSession(config(&window{}, &panel{copyFail: errBroken}, 4))
	s.Ready()

	s.Report(said("Oui.", "Да."))
	s.Choose(controls.Choice{Command: controls.CopyLast})

	if err := s.Close(); !errors.Is(err, errBroken) {
		t.Errorf("closing gave %v, want the panel's failure", err)
	}
}

func TestMovingShowsTheHintUnderThePairsAndKeepsWhereItEnded(t *testing.T) {
	w, p := &window{corner: ui.Point{X: 300, Y: 700}}, &panel{}
	var kept []ui.Point
	c := config(w, p, 4)
	c.Place = func(at ui.Point) error {
		kept = append(kept, at)

		return nil
	}
	s := NewSession(c)
	s.Ready()
	for _, text := range []string{"Un.", "Deux.", "Oui."} {
		s.Report(said(text, "?"))
	}
	s.Choose(controls.Choice{Command: controls.ToggleHidden})

	s.Choose(controls.Choice{Command: controls.ToggleMove})
	want := []ui.Pair{{Original: "Un.", Translation: "?"}, {Original: "Deux.", Translation: "?"}, {Original: "Oui.", Translation: "?"}, {Original: "Drag me"}}
	if got := w.last(t); !got.Moving || got.Hidden || !slices.Equal(got.Pairs, want) || !p.last(t).Moving || len(kept) != 0 {
		t.Errorf("moving shows %+v with %d corners kept; want the pair, the hint, in sight, and nothing kept yet", got, len(kept))
	}

	moving := len(w.frames) - 1

	s.Choose(controls.Choice{Command: controls.ToggleMove})
	if got := w.last(t); got.Moving || !got.Hidden || len(got.Pairs) != 3 || !slices.Equal(kept, []ui.Point{{X: 300, Y: 700}}) {
		t.Errorf("fixed, it shows %+v and kept %v; want the pairs alone, hidden again, and the corner kept", got, kept)
	}

	s.Report(said("Non.", "Нет."))
	if got := w.frames[moving].Pairs; !slices.Equal(got, want) {
		t.Errorf("a frame already shown changed to %v, want it left as %v", got, want)
	}
}

func TestACornerThatCannotBeKeptIsTold(t *testing.T) {
	p := &panel{}
	c := config(&window{}, p, 4)
	c.Place = func(ui.Point) error { return errReadOnly }
	s := NewSession(c)

	s.Choose(controls.Choice{Command: controls.ToggleMove})
	s.Choose(controls.Choice{Command: controls.ToggleMove})

	if len(p.alerts) != 1 || p.alerts[0] != errReadOnly.Error() {
		t.Errorf("alerts %q, want the failure told", p.alerts)
	}
}

func TestSettingsOpenWithAWayBackForWhatChangesAtOnce(t *testing.T) {
	c := config(&window{}, &panel{}, 4)
	opened := 0
	c.Configure = func(adopt func(Live)) error {
		opened++
		adopt(Live{Language: "en", Target: "ru", Shown: 4})

		return nil
	}
	s := NewSession(c)

	s.Choose(controls.Choice{Command: controls.OpenSettings})

	if opened != 1 || s.Language() != "en" {
		t.Errorf("settings opened %d times and the session speaks %q, want once and en", opened, s.Language())
	}
}

func TestSettingsThatCannotOpenAreTold(t *testing.T) {
	p := &panel{}
	c := config(&window{}, p, 4)
	c.Configure = func(func(Live)) error { return errReadOnly }

	NewSession(c).Choose(controls.Choice{Command: controls.OpenSettings})
	NewSession(config(&window{}, p, 4)).Choose(controls.Choice{Command: controls.OpenSettings})

	if len(p.alerts) != 1 || p.alerts[0] != errReadOnly.Error() {
		t.Errorf("alerts %q, want the failure told once and nothing from a session without settings", p.alerts)
	}
}

func TestAdoptedSettingsTakeHoldAtOnce(t *testing.T) {
	w, p := &window{}, &panel{}
	c := config(w, p, 4)
	remembered := 0
	c.Remember = func(string) error {
		remembered++

		return nil
	}
	s := NewSession(c)
	s.Ready()
	for _, text := range []string{"Un.", "Deux.", "Trois."} {
		s.Report(said(text, ""))
	}

	look := ui.Look{TextPoints: 16, WidthPercent: 40, BackgroundPercent: 50}
	s.Adopt(Live{Language: "en", Target: "en", Look: look, Shown: 2})
	adopted := originals(w.last(t))

	s.Report(said("Quatre.", ""))
	later := originals(w.last(t))

	if !slices.Equal(adopted, []string{"Deux.", "Trois."}) || !slices.Equal(later, []string{"Trois.", "Quatre."}) {
		t.Errorf("shows %q and then %q, want the two newest pairs each time", adopted, later)
	}

	if !slices.Equal(w.looks, []ui.Look{look}) {
		t.Errorf("the window got the looks %+v, want %+v", w.looks, look)
	}

	if status := p.last(t); status.Language != "en" || status.Target != "en" || remembered != 0 {
		t.Errorf("the panel shows %s → %s and %d languages were remembered, want en → en and none, the file holds them already", status.Language, status.Target, remembered)
	}
}

func TestAClosedSessionAdoptsNothing(t *testing.T) {
	w := &window{}
	s := NewSession(config(w, &panel{}, 4))
	s.Close()

	s.Adopt(Live{Language: "en", Target: "en", Shown: 2})

	if len(w.looks) != 0 || s.Language() != "fr" {
		t.Errorf("a closed session restyled its window %d times and speaks %q, want nothing changed", len(w.looks), s.Language())
	}
}

func TestAnAdoptedTargetGoesToTheNextTranslation(t *testing.T) {
	s := NewSession(config(&window{}, &panel{}, 4))

	var targets []string
	translator := s.Translator(func(_, target string) translate.Translator {
		targets = append(targets, target)

		return words("Yes.")
	})

	translator.Translate(context.Background(), "Oui.")
	s.Adopt(Live{Language: "fr", Target: "en", Shown: 4})
	translator.Translate(context.Background(), "Oui.")

	if !slices.Equal(targets, []string{"ru", "en"}) {
		t.Errorf("translated into %q, want ru and then en", targets)
	}
}

func TestAnAdoptedLingerSendsOlderPairsAway(t *testing.T) {
	w, at := &window{}, &clock{now: time.Unix(1000, 0)}
	c := config(w, &panel{}, 4)
	c.Now = at.read
	s := NewSession(c)
	s.Ready()

	s.Report(said("Oui.", "Да."))
	at.pass(5 * time.Second)
	s.Expire()
	s.Adopt(Live{Language: "fr", Target: "ru", Shown: 4, Linger: 3 * time.Second})
	s.Report(said("Non.", "Нет."))
	at.pass(time.Second)
	s.Expire()

	if got := originals(w.last(t)); !slices.Equal(got, []string{"Non."}) {
		t.Errorf("shows %q, want Non. alone: Oui. is past the new three seconds and Non. is not", got)
	}
}

func TestAnAdoptedLanguageSublayerDoesNotKnowLeavesTheOneSpoken(t *testing.T) {
	s := NewSession(config(&window{}, &panel{}, 4))

	s.Adopt(Live{Language: "de", Target: "ru", Shown: 4})

	if got := s.Language(); got != "fr" {
		t.Errorf("the session speaks %q, want fr kept", got)
	}
}

func TestALookTheWindowCannotTakeEndsTheSession(t *testing.T) {
	w := &window{styleFail: errBroken}
	s := NewSession(config(w, &panel{}, 4))

	s.Adopt(Live{Language: "fr", Target: "ru", Shown: 4})

	select {
	case <-s.Context().Done():
	default:
		t.Error("the session goes on after the window refused the look")
	}

	if err := s.Close(); !errors.Is(err, errBroken) {
		t.Errorf("closing tells %v, want the window's failure", err)
	}
}

type clock struct {
	now time.Time
}

func (c *clock) read() time.Time {
	return c.now
}

func (c *clock) pass(d time.Duration) {
	c.now = c.now.Add(d)
}

func TestPairsLeaveOnceTheyLingeredTheirTime(t *testing.T) {
	w, at := &window{}, &clock{now: time.Unix(1000, 0)}
	c := config(w, &panel{}, 4)
	c.Linger, c.Now = 10*time.Second, at.read
	s := NewSession(c)
	s.Ready()

	s.Report(said("Un.", "Один."))
	at.pass(6 * time.Second)
	s.Report(said("Deux.", "Два."))

	shown := func() []string {
		var texts []string
		for _, p := range w.last(t).Pairs {
			texts = append(texts, p.Original)
		}

		return texts
	}

	at.pass(3 * time.Second)
	frames := len(w.frames)
	s.Expire()
	if len(w.frames) != frames || !slices.Equal(shown(), []string{"Un.", "Deux."}) {
		t.Errorf("at 9 s got %q in %d new frames, want both pairs and no new frame", shown(), len(w.frames)-frames)
	}

	at.pass(time.Second)
	s.Expire()
	if !slices.Equal(shown(), []string{"Deux."}) {
		t.Errorf("at 10 s got %q, want Deux. alone", shown())
	}

	at.pass(6 * time.Second)
	s.Expire()
	if got := shown(); len(got) != 0 {
		t.Errorf("at 16 s got %q, want nothing", got)
	}
}

func TestALingerOfZeroKeepsPairsUntilNewerOnesPushThemOut(t *testing.T) {
	w, at := &window{}, &clock{now: time.Unix(1000, 0)}
	c := config(w, &panel{}, 4)
	c.Now = at.read
	s := NewSession(c)
	s.Ready()

	s.Report(said("Oui.", "Да."))
	at.pass(time.Hour)
	s.Expire()

	if got := len(w.last(t).Pairs); got != 1 {
		t.Errorf("an hour later %d pairs show, want the one kept", got)
	}
}

func TestQuitEndsTheSessionOnceHoweverOftenItIsChosen(t *testing.T) {
	s := NewSession(config(&window{}, &panel{}, 4))

	s.Choose(controls.Choice{Command: controls.Quit})
	s.Choose(controls.Choice{Command: controls.Quit})

	select {
	case <-s.Context().Done():
	default:
		t.Error("the session went on after quitting")
	}
}

func TestFollowTakesEveryChoiceUntilTheyEnd(t *testing.T) {
	w := &window{}
	s := listening(w, 4)
	choices := make(chan controls.Choice, 2)
	choices <- controls.Choice{Command: controls.TogglePause}
	choices <- controls.Choice{Command: controls.ToggleHidden}
	close(choices)

	s.Follow(choices)

	if got := w.last(t); got.State != ui.Paused || !got.Hidden {
		t.Errorf("got %+v, want paused and hidden", got)
	}
}

func TestSessionEndsOnceTheWindowFails(t *testing.T) {
	w := &window{fail: errBroken}
	s := NewSession(config(w, &panel{}, 4))

	s.Report(said("Oui.", "Да."))

	select {
	case <-s.Context().Done():
	default:
		t.Error("the session went on with a broken window")
	}

	if err := s.Close(); !errors.Is(err, errBroken) || !w.closed {
		t.Errorf("closing gave %v (window closed %v), want the window's failure and the window closed", err, w.closed)
	}
}

func TestSessionEndsOnceThePanelFails(t *testing.T) {
	p := &panel{fail: errBroken}
	s := NewSession(config(&window{}, p, 4))

	select {
	case <-s.Context().Done():
	default:
		t.Error("the session went on with a broken panel")
	}

	if err := s.Close(); !errors.Is(err, errBroken) || !p.closed {
		t.Errorf("closing gave %v (panel closed %v), want the panel's failure and the panel closed", err, p.closed)
	}
}

func TestTheFirstFailureIsTheOneClosingTells(t *testing.T) {
	w := &window{fail: errBroken}
	c := config(w, &panel{alertFail: errReadOnly}, 4)
	c.Remember = func(string) error { return errReadOnly }
	s := NewSession(c)

	s.Choose(controls.Choice{Command: controls.ChooseLanguage, Language: "en"})

	if err := s.Close(); !errors.Is(err, errBroken) {
		t.Errorf("closing gave %v, want the window's failure that came first", err)
	}
}

func TestSessionShowsNothingOnceClosed(t *testing.T) {
	w, p := &window{}, &panel{}
	s := NewSession(config(w, p, 4))
	s.Ready()

	s.Close()
	frames, statuses := len(w.frames), len(p.statuses)
	s.Report(said("Oui.", "Да."))
	s.Choose(controls.Choice{Command: controls.TogglePause})

	if len(w.frames) != frames || len(p.statuses) != statuses {
		t.Errorf("after closing, the window got %d more frames and the panel %d more statuses, want none", len(w.frames)-frames, len(p.statuses)-statuses)
	}
}

func TestSessionClosesTheWindowAndPassesOnWhatItSays(t *testing.T) {
	w := &window{closeErr: errBroken}
	s := listening(w, 4)

	s.Report(said("Oui.", "Да."))

	if err := s.Close(); !errors.Is(err, errBroken) || !w.closed {
		t.Errorf("closing gave %v (window closed %v), want the window closed and its own answer", err, w.closed)
	}
}

func TestSessionGoesOnWithoutAPanel(t *testing.T) {
	w := &window{}
	c := config(w, nil, 4)
	c.Panel = nil
	c.Remember = func(string) error { return errReadOnly }
	s := NewSession(c)
	s.Ready()

	s.Report(said("Oui.", "Да."))
	s.Choose(controls.Choice{Command: controls.ChooseLanguage, Language: "en"})

	if err := s.Close(); err != nil || len(w.last(t).Pairs) != 1 || s.Language() != "en" {
		t.Errorf("closing gave %v with %+v in %q, want the pair shown in en and a clean close", err, w.last(t), s.Language())
	}
}

func TestSessionTakesNewsAndChoicesFromSeveralGoroutinesAtOnce(t *testing.T) {
	w := &window{}
	s := listening(w, 3)
	source := s.Listen(&speakers{chunks: make([][]float32, 200)})

	var done sync.WaitGroup
	for range 4 {
		done.Add(1)
		go func() {
			defer done.Done()

			for i := range 50 {
				s.Recognizing(vad.Phrase{})
				s.Report(said("Oui.", "Да."))
				s.Choose(controls.Choice{Command: controls.ToggleHidden})
				s.Choose(controls.Choice{Command: controls.TogglePause})
				s.Choose(controls.Choice{Command: controls.ChooseLanguage, Language: []string{"en", "fr"}[i%2]})
				s.Language()
			}
		}()
	}

	done.Add(1)
	go func() {
		defer done.Done()

		for range 200 {
			source.Read()
		}
	}()
	done.Wait()

	if got := len(w.last(t).Pairs); got != 3 {
		t.Errorf("the last frame holds %d pairs, want the 3 shown", got)
	}
}

type speech string

func (s speech) Transcribe(context.Context, []float32, int) (string, error) {
	return string(s), nil
}

type words string

func (w words) Translate(context.Context, string) (string, error) {
	return string(w), nil
}
