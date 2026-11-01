package app

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/translate"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

var errBroken = errors.New("ui: the overlay window failed")

type window struct {
	frames   []ui.Frame
	fail     error
	closeErr error
	closed   bool
}

func (w *window) Show(f ui.Frame) error {
	if w.fail != nil {
		return w.fail
	}

	w.frames = append(w.frames, f)

	return nil
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

func subtitlesOn(w *window, shown int) *Subtitles {
	return NewSubtitles(Config{Window: w, Shown: shown, StopListening: func() {}})
}

func said(text, translation string) pipeline.Result {
	return pipeline.Result{Text: text, Translation: translation}
}

func TestSubtitlesPutEachNewPairUnderTheOnesBefore(t *testing.T) {
	w := &window{}
	s := subtitlesOn(w, 4)

	s.Report(said("Bonjour.", "Здравствуйте."))
	s.Report(said("Merci.", "Спасибо."))

	want := []ui.Pair{{Original: "Bonjour.", Translation: "Здравствуйте."}, {Original: "Merci.", Translation: "Спасибо."}}
	if got := w.last(t); !slices.Equal(got.Pairs, want) || got.State != ui.Listening {
		t.Errorf("got %v in state %d, want %v while listening", got.Pairs, got.State, want)
	}
}

func TestSubtitlesKeepOnlyAsManyPairsAsAreShown(t *testing.T) {
	w := &window{}
	s := subtitlesOn(w, 2)

	for _, text := range []string{"Un.", "Deux.", "Trois."} {
		s.Report(said(text, "?"))
	}

	if got := w.last(t).Pairs; len(got) != 2 || got[0].Original != "Deux." || got[1].Original != "Trois." {
		t.Errorf("got %v, want the latest two, Deux. then Trois.", got)
	}
}

func TestSubtitlesLightTheDotWhileAPhraseIsRecognized(t *testing.T) {
	w := &window{}
	s := subtitlesOn(w, 4)

	s.Recognizing(vad.Phrase{})
	if got := w.last(t).State; got != ui.Recognizing {
		t.Errorf("a phrase in recognition shows state %d, want recognizing", got)
	}

	s.Report(said("Oui.", "Да."))
	if got := w.last(t).State; got != ui.Listening {
		t.Errorf("a recognized phrase leaves state %d, want listening again", got)
	}
}

func TestSubtitlesShowAFailureUntilAPhraseComesThrough(t *testing.T) {
	w := &window{}
	s := subtitlesOn(w, 4)

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

func TestSubtitlesShowTheOriginalOfAPhraseLeftUntranslated(t *testing.T) {
	w := &window{}
	s := subtitlesOn(w, 4)

	s.Report(pipeline.Result{Text: "Oui.", Err: translate.ErrNoAnswer})

	want := []ui.Pair{{Original: "Oui."}}
	if got := w.last(t); !slices.Equal(got.Pairs, want) || got.State != ui.Failed {
		t.Errorf("got %v in state %d, want the original alone and the failure", got.Pairs, got.State)
	}
}

func TestSubtitlesLeaveOutAPhraseWithoutWords(t *testing.T) {
	w := &window{}
	s := subtitlesOn(w, 4)

	s.Report(said("Oui.", "Да."))
	s.Recognizing(vad.Phrase{})
	s.Report(pipeline.Result{})

	if got := w.last(t); len(got.Pairs) != 1 || got.State != ui.Listening {
		t.Errorf("got %v in state %d, want the one pair and listening", got.Pairs, got.State)
	}
}

func TestSubtitlesLeaveTheDotAloneForASkippedPhrase(t *testing.T) {
	w := &window{}
	s := subtitlesOn(w, 4)

	s.Recognizing(vad.Phrase{})
	s.Report(pipeline.Result{Err: pipeline.ErrSkipped})

	if got := w.last(t); got.State != ui.Recognizing || len(got.Pairs) != 0 {
		t.Errorf("a skipped phrase left %v in state %d, want recognition still showing", got.Pairs, got.State)
	}
}

func TestSubtitlesStopListeningOnceTheWindowFails(t *testing.T) {
	w := &window{fail: errBroken}
	stops := 0
	s := NewSubtitles(Config{Window: w, Shown: 4, StopListening: func() { stops++ }})

	s.Report(said("Oui.", "Да."))
	s.Report(said("Non.", "Нет."))

	if stops != 1 {
		t.Errorf("the listening was stopped %d times, want once at the first failure", stops)
	}

	if err := s.Close(); !errors.Is(err, errBroken) || !w.closed {
		t.Errorf("closing gave %v (window closed %v), want the window's failure and the window closed", err, w.closed)
	}
}

func TestSubtitlesCloseTheWindowAndPassOnWhatItSays(t *testing.T) {
	w := &window{closeErr: errBroken}
	s := subtitlesOn(w, 4)

	s.Report(said("Oui.", "Да."))

	if err := s.Close(); !errors.Is(err, errBroken) || !w.closed {
		t.Errorf("closing gave %v (window closed %v), want the window closed and its own answer", err, w.closed)
	}
}

func TestSubtitlesTakeNewsFromSeveralGoroutinesAtOnce(t *testing.T) {
	w := &window{}
	s := subtitlesOn(w, 3)

	var done sync.WaitGroup
	for range 4 {
		done.Add(1)
		go func() {
			defer done.Done()

			for range 50 {
				s.Recognizing(vad.Phrase{})
				s.Report(said("Oui.", "Да."))
			}
		}()
	}
	done.Wait()

	if got := len(w.last(t).Pairs); got != 3 {
		t.Errorf("the last frame holds %d pairs, want the 3 shown", got)
	}
}
