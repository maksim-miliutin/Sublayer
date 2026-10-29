package pipeline

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/stt"
	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

const rate = 22050

func quiet(seconds float64) []float32 {
	return make([]float32, int(seconds*rate))
}

func voice(seconds float64) []float32 {
	out := make([]float32, int(seconds*rate))
	for i := range out {
		out[i] = float32(0.3 * math.Sin(2*math.Pi*300*float64(i)/rate))
	}

	return out
}

type source struct {
	pieces [][]float32
	end    error
	before func(piece int) // runs as each piece is asked for, the end counting as one more
	next   int
}

func (s *source) Read() ([]float32, error) {
	if s.before != nil {
		s.before(s.next)
	}

	if s.next == len(s.pieces) {
		return nil, s.end
	}

	s.next++

	return s.pieces[s.next-1], nil
}

func (s *source) Close() error {
	return nil
}

type transcriber func(speech []float32, rate int) (string, error)

func (f transcriber) Transcribe(_ context.Context, speech []float32, rate int) (string, error) {
	return f(speech, rate)
}

func lengthOf(speech []float32, _ int) (string, error) {
	return strconv.Itoa(len(speech)), nil
}

func run(t *testing.T, c Config) ([]Result, error) {
	t.Helper()

	var results []Result
	c.Report = func(r Result) { results = append(results, r) }
	c.Rate = rate
	if c.Phrases == (vad.Config{}) {
		c.Phrases = vad.Defaults()
	}

	err := Run(context.Background(), c)

	return results, err
}

// Phrases follow one another, and each holds as many samples as its times span at the
// source's rate, which is how a test sees that the cutting ran at that rate.
func inSourceTime(t *testing.T, results []Result) {
	t.Helper()

	for i, r := range results {
		span := int(math.Round((r.Phrase.End - r.Phrase.Start).Seconds() * rate))
		if len(r.Phrase.Samples) != span {
			t.Errorf("result %d holds %d samples for %v to %v, want the %d those times span", i, len(r.Phrase.Samples), r.Phrase.Start, r.Phrase.End, span)
		}

		if i > 0 && r.Phrase.Start <= results[i-1].Phrase.End {
			t.Errorf("result %d starts at %v, before the one ahead of it ends at %v", i, r.Phrase.Start, results[i-1].Phrase.End)
		}
	}
}

func threePhrases() []float32 {
	return slices.Concat(quiet(0.5), voice(1), quiet(1), voice(0.6), quiet(1), voice(1.4), quiet(1))
}

func TestRunRecognizesEveryPhraseInTheOrderItWasSaid(t *testing.T) {
	var rates []int
	results, err := run(t, Config{
		Source: &source{pieces: slices.Collect(slices.Chunk(threePhrases(), 160)), end: capture.ErrClosed},
		Transcriber: transcriber(func(speech []float32, r int) (string, error) {
			rates = append(rates, r)

			return lengthOf(speech, r)
		}),
	})

	if !errors.Is(err, capture.ErrClosed) {
		t.Errorf("got %v, want the source's own end", err)
	}

	if len(results) != 3 {
		t.Fatalf("got %d results, want one for each of the 3 phrases", len(results))
	}

	for i, r := range results {
		if r.Err != nil || r.Text != strconv.Itoa(len(r.Phrase.Samples)) {
			t.Errorf("phrase %d of %d samples came back as %q, %v", i, len(r.Phrase.Samples), r.Text, r.Err)
		}
	}
	inSourceTime(t, results)

	if !slices.Equal(rates, []int{rate, rate, rate}) {
		t.Errorf("the recognizer heard the phrases at %v Hz, want the source's %d", rates, rate)
	}
}

func TestRunCutsPhrasesTheWayItIsTold(t *testing.T) {
	phrases := vad.Defaults()
	phrases.MinSpeech = 800 * time.Millisecond

	results, _ := run(t, Config{
		Source:      &source{pieces: [][]float32{threePhrases()}, end: capture.ErrClosed},
		Transcriber: transcriber(lengthOf),
		Phrases:     phrases,
	})

	if len(results) != 2 {
		t.Errorf("got %d phrases, want 2 with the 0.6 s one too short for 0.8 s of speech", len(results))
	}
}

// The first phrase holds recognition up until every later phrase is cut, so three
// phrases arrive while one is being recognized.
func behind(t *testing.T, waiting int) []Result {
	t.Helper()

	busy, fed := make(chan struct{}), make(chan struct{})
	first := slices.Concat(quiet(0.5), voice(0.6), quiet(1))
	rest := slices.Concat(voice(0.8), quiet(1), voice(1), quiet(1), voice(1.2), quiet(1))

	calls := 0
	results, err := run(t, Config{
		Source: &source{pieces: [][]float32{first, rest}, end: capture.ErrClosed, before: func(piece int) {
			switch piece {
			case 1:
				<-busy
			case 2:
				close(fed)
			}
		}},
		Transcriber: transcriber(func(speech []float32, r int) (string, error) {
			calls++
			if calls == 1 {
				close(busy)
				<-fed
			}

			return lengthOf(speech, r)
		}),
		Waiting: waiting,
	})
	if !errors.Is(err, capture.ErrClosed) {
		t.Fatalf("got %v, want the source's own end", err)
	}

	inSourceTime(t, results)

	return results
}

func TestRunSkipsPhrasesThatWaitedWhileRecognitionWasBehind(t *testing.T) {
	results := behind(t, 1)

	want := []error{nil, ErrSkipped, ErrSkipped, nil}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}

	for i, r := range results {
		if !errors.Is(r.Err, want[i]) {
			t.Errorf("phrase %d came back with %v, want %v", i, r.Err, want[i])
		}

		if r.Err == nil && r.Text != strconv.Itoa(len(r.Phrase.Samples)) {
			t.Errorf("phrase %d of %d samples was recognized as %q", i, len(r.Phrase.Samples), r.Text)
		}
	}
}

func TestRunWithoutAWaitingLimitRecognizesEveryPhrase(t *testing.T) {
	results := behind(t, 0)

	if len(results) != 4 {
		t.Fatalf("got %d results, want all 4 phrases", len(results))
	}

	for i, r := range results {
		if r.Err != nil {
			t.Errorf("phrase %d came back with %v, want it recognized", i, r.Err)
		}
	}
}

func TestRunRecognizesThePhraseTheSourceCutShort(t *testing.T) {
	failure := fmt.Errorf("%w: unplugged", capture.ErrDevice)

	results, err := run(t, Config{
		Source:      &source{pieces: [][]float32{slices.Concat(quiet(0.5), voice(1))}, end: failure},
		Transcriber: transcriber(lengthOf),
	})

	if !errors.Is(err, capture.ErrDevice) {
		t.Errorf("got %v, want the device failure", err)
	}

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("got %v, want the phrase that was still going recognized", results)
	}

	if p := results[0].Phrase; p.End-p.Start < time.Second {
		t.Errorf("the phrase runs %v to %v, want the whole second of voice", p.Start, p.End)
	}
}

func TestRunReportsAFailedRecognitionAndGoesOnToTheNext(t *testing.T) {
	audio := slices.Concat(quiet(0.5), voice(0.6), quiet(1), voice(0.6), quiet(1))

	calls := 0
	results, _ := run(t, Config{
		Source: &source{pieces: [][]float32{audio}, end: capture.ErrClosed},
		Transcriber: transcriber(func([]float32, int) (string, error) {
			calls++
			if calls == 1 {
				return "", stt.ErrNoAnswer
			}

			return "Oui.", nil
		}),
	})

	if len(results) != 2 || !errors.Is(results[0].Err, stt.ErrNoAnswer) || results[1].Err != nil || results[1].Text != "Oui." {
		t.Errorf("got %v, want the failure and then Oui.", results)
	}
}

func TestRunTellsHowLongEachRecognitionTook(t *testing.T) {
	results, _ := run(t, Config{
		Source: &source{pieces: [][]float32{slices.Concat(quiet(0.5), voice(0.6), quiet(1))}, end: capture.ErrClosed},
		Transcriber: transcriber(func([]float32, int) (string, error) {
			time.Sleep(30 * time.Millisecond)

			return "Oui.", nil
		}),
	})

	if len(results) != 1 || results[0].Took < 30*time.Millisecond {
		t.Errorf("got %v, want one result that took at least the 30 ms spent", results)
	}
}
