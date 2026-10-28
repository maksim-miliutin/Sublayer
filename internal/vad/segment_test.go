package vad

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

const rate = 16000

type piece struct {
	span time.Duration
	kind string
	gain float64
}

func silence(d time.Duration) piece            { return piece{d, "silence", 0} }
func speech(d time.Duration) piece             { return piece{d, "speech", 0.3} }
func murmur(d time.Duration) piece             { return piece{d, "speech", 0.00154} }
func whisper(d time.Duration) piece            { return piece{d, "speech", 0.01} }
func hum(d time.Duration) piece                { return piece{d, "hum", 0} }
func speechOverHum(d time.Duration) piece      { return piece{d, "speech+hum", 0.3} }
func samplesIn(d time.Duration) int            { return int(d * rate / time.Second) }
func near(got, want time.Duration) bool        { return (got - want).Abs() <= frameSpan }
func secs(s float64) time.Duration             { return time.Duration(s * float64(time.Second)) }
func noise(r *rand.Rand, gain float64) float32 { return float32(gain * (2*r.Float64() - 1)) }

// Speech is modelled as noise with a syllable dip of 20 dB every 260 ms, ending loud,
// because steady noise would sink into the floor the way a hum does.
func signal(pieces ...piece) []float32 {
	r := rand.New(rand.NewPCG(5, 6))
	var out []float32
	for _, p := range pieces {
		n := samplesIn(p.span)
		cycle, loud := samplesIn(260*time.Millisecond), samplesIn(200*time.Millisecond)
		for i := range n {
			var s float64
			if p.kind == "hum" || p.kind == "speech+hum" {
				s += 0.025 * math.Sqrt2 * math.Sin(2*math.Pi*100*float64(len(out))/rate)
			}

			if p.kind == "speech" || p.kind == "speech+hum" {
				gain := p.gain
				if i%cycle >= loud && i < n-loud {
					gain /= 10
				}
				s += float64(noise(r, gain))
			}

			out = append(out, float32(s))
		}
	}

	return out
}

func run(samples []float32) []Phrase {
	s := New(rate, Defaults())

	return append(s.Feed(samples), s.Flush()...)
}

func TestSegmenterFindsAPhraseBetweenSilences(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(2)), silence(secs(1)))

	got := run(in)
	if len(got) != 1 {
		t.Fatalf("got %d phrases, want 1", len(got))
	}

	p := got[0]
	if !near(p.Start, secs(1)-preRoll) || !near(p.End, secs(3)+postRoll) {
		t.Errorf("phrase runs %v to %v, want about %v to %v", p.Start, p.End, secs(1)-preRoll, secs(3)+postRoll)
	}

	from, to := samplesIn(p.Start), samplesIn(p.End)
	if !slices.Equal(p.Samples, in[from:to]) {
		t.Error("the phrase does not carry exactly the samples between its start and end")
	}
}

func TestSegmenterKeepsWordsWithShortGapsTogether(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(0.8)), silence(secs(0.3)), speech(secs(0.8)), silence(secs(1)))

	if got := run(in); len(got) != 1 {
		t.Errorf("got %d phrases across a 300 ms gap, want 1", len(got))
	}
}

func TestSegmenterSplitsAtAPauseLongerThanTheEndPause(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(0.8)), silence(secs(0.8)), speech(secs(0.8)), silence(secs(1)))

	got := run(in)
	if len(got) != 2 {
		t.Fatalf("got %d phrases across an 800 ms pause, want 2", len(got))
	}

	if got[0].End > got[1].Start {
		t.Errorf("the first phrase ends at %v, after the second starts at %v", got[0].End, got[1].Start)
	}
}

func TestSegmenterEndsAPhraseAfterExactlyTheEndPause(t *testing.T) {
	for _, c := range []struct {
		gap  time.Duration
		want int
	}{
		{600 * time.Millisecond, 2},
		{580 * time.Millisecond, 1},
	} {
		in := signal(silence(secs(1)), speech(secs(0.8)), silence(c.gap), speech(secs(0.8)), silence(secs(1)))

		if got := run(in); len(got) != c.want {
			t.Errorf("a %v pause gave %d phrases, want %d", c.gap, len(got), c.want)
		}
	}
}

// Murmur sits about 9 dB over digital silence: too quiet to open a phrase, loud enough
// to keep one open, the way the soft end of a word trails off.
func TestSegmenterHoldsOnToASoftEndingOnceAPhraseIsOpen(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(0.5)), murmur(secs(0.8)), silence(secs(1)))

	got := run(in)
	if len(got) != 1 {
		t.Fatalf("got %d phrases, want one running through the murmur", len(got))
	}

	if !near(got[0].End, secs(2.3)+postRoll) {
		t.Errorf("the phrase ends at %v, want about %v after the murmur", got[0].End, secs(2.3)+postRoll)
	}

	if alone := run(signal(silence(secs(1)), murmur(secs(1)), silence(secs(1)))); len(alone) != 0 {
		t.Errorf("murmur alone opened %d phrases, want none", len(alone))
	}
}

func TestSegmenterJudgesEachPhraseOnItsOwnSpeech(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(1)), silence(secs(1)), speech(secs(0.1)), silence(secs(1)))

	if got := run(in); len(got) != 1 {
		t.Errorf("got %d phrases, want the click after a real phrase dropped like any other click", len(got))
	}
}

func TestSegmenterIgnoresSoundsShorterThanTheMinimumSpeech(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(0.15)), silence(secs(1)))

	if got := run(in); len(got) != 0 {
		t.Errorf("got %d phrases from a 150 ms click, want none", len(got))
	}
}

func TestSegmenterIgnoresASteadyHumButHearsSpeechOverIt(t *testing.T) {
	if got := run(signal(hum(secs(5)))); len(got) != 0 {
		t.Errorf("got %d phrases from a steady hum, want none", len(got))
	}

	got := run(signal(hum(secs(4)), speechOverHum(secs(2)), hum(secs(2))))
	if len(got) != 1 || !near(got[0].Start, secs(4)-preRoll) {
		t.Errorf("got %d phrases over the hum; want one starting at about %v", len(got), secs(4)-preRoll)
	}
}

func TestSegmenterHearsQuietSpeechInDigitalSilence(t *testing.T) {
	in := signal(silence(secs(1)), whisper(secs(1)), silence(secs(1)))

	if got := run(in); len(got) != 1 {
		t.Errorf("got %d phrases from speech at about -45 dB, want 1", len(got))
	}
}

func TestSegmenterStartsNoEarlierThanTheStream(t *testing.T) {
	got := run(signal(silence(secs(0.1)), speech(secs(1)), silence(secs(1))))

	if len(got) != 1 || got[0].Start != 0 {
		t.Errorf("got %v, want one phrase whose pre-roll stops at the stream start", got)
	}
}

func TestSegmenterGivesTheSamePhrasesWhateverTheChunks(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(1.5)), silence(secs(0.9)), speech(secs(2)), silence(secs(0.2)))
	whole := run(in)

	r := rand.New(rand.NewPCG(7, 8))
	s := New(rate, Defaults())
	var pieces []Phrase
	for rest := in; len(rest) > 0; {
		n := min(r.IntN(900), len(rest))
		pieces = append(pieces, s.Feed(rest[:n])...)
		rest = rest[n:]
	}
	pieces = append(pieces, s.Flush()...)

	if len(whole) != 2 || len(pieces) != len(whole) {
		t.Fatalf("got %d phrases fed whole and %d fed in chunks, want 2 each", len(whole), len(pieces))
	}

	for i := range whole {
		if whole[i].Start != pieces[i].Start || whole[i].End != pieces[i].End || !slices.Equal(whole[i].Samples, pieces[i].Samples) {
			t.Errorf("phrase %d differs: %v-%v whole, %v-%v chunked", i, whole[i].Start, whole[i].End, pieces[i].Start, pieces[i].End)
		}
	}
}

func TestSegmenterFlushesAPhraseTheStreamCutShort(t *testing.T) {
	s := New(rate, Defaults())
	if got := s.Feed(signal(silence(secs(1)), speech(secs(1)))); len(got) != 0 {
		t.Fatalf("got %d phrases before the pause, want none yet", len(got))
	}

	got := s.Flush()
	if len(got) != 1 || !near(got[0].End, secs(2)) {
		t.Errorf("flush gave %v, want one phrase ending with the stream at about 2s", got)
	}

	if again := s.Flush(); len(again) != 0 {
		t.Errorf("a second flush gave %d more phrases", len(again))
	}

	s = New(rate, Defaults())
	s.Feed(signal(silence(secs(1)), speech(secs(1)), silence(secs(0.4))))
	if got := s.Flush(); len(got) != 1 || !near(got[0].End, secs(2)+postRoll) {
		t.Errorf("flush in a short pause gave %v, want one phrase ending at about %v, not with the stream", got, secs(2)+postRoll)
	}
}

func TestSegmenterFlushDropsAClickTheStreamCutShort(t *testing.T) {
	s := New(rate, Defaults())
	s.Feed(signal(silence(secs(1)), speech(secs(0.1))))

	if got := s.Flush(); len(got) != 0 {
		t.Errorf("flush gave %d phrases from a 100 ms click, want none", len(got))
	}
}

func TestPhraseTimesMatchItsSamples(t *testing.T) {
	for _, p := range run(signal(silence(secs(1)), speech(secs(1.3)), silence(secs(1)), speech(secs(0.7)), silence(secs(1)))) {
		if want := time.Duration(len(p.Samples)) * time.Second / rate; p.End-p.Start != want {
			t.Errorf("phrase %v-%v lasts %v but holds %v of samples", p.Start, p.End, p.End-p.Start, want)
		}
	}
}

func TestSegmenterCutsLongSpeechInsideItsQuietestRecentPause(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(9.5)), silence(secs(0.1)), speech(secs(10.4)), silence(secs(1)))

	got := run(in)
	if len(got) != 2 {
		t.Fatalf("got %d phrases, want the long speech cut in two", len(got))
	}

	if cut := got[0].End; cut < secs(10.5) || cut >= secs(10.6) {
		t.Errorf("cut at %v, want inside the short pause, at or after 10.5s and before speech resumes at 10.6s", cut)
	}

	if got[1].Start != got[0].End {
		t.Errorf("the second part starts at %v, the first ends at %v; want them to meet", got[1].Start, got[0].End)
	}
}

func TestSegmenterCutsEndlessSpeechIntoPhrasesNoLongerThanTheMax(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(40)), silence(secs(1)))

	got := run(in)
	if len(got) < 4 {
		t.Fatalf("got %d phrases from 40 s of speech, want at least 4", len(got))
	}

	var joined []float32
	for i, p := range got {
		if p.End-p.Start > Defaults().MaxPhrase {
			t.Errorf("phrase %d lasts %v, longer than the %v allowed", i, p.End-p.Start, Defaults().MaxPhrase)
		}

		if i > 0 && p.Start != got[i-1].End {
			t.Errorf("phrase %d starts at %v, the one before ends at %v", i, p.Start, got[i-1].End)
		}

		if i < len(got)-1 && p.End-p.Start < Defaults().MaxPhrase-cutReach {
			t.Errorf("phrase %d was cut after only %v, want the cut within the last %v before the limit", i, p.End-p.Start, cutReach)
		}

		joined = append(joined, p.Samples...)
	}

	first, last := got[0].Start, got[len(got)-1].End
	if !slices.Equal(joined, in[samplesIn(first):samplesIn(last)]) {
		t.Error("the cut phrases put together are not the speech they were cut from")
	}
}

func TestSegmenterCutsTheSameWhateverTheChunks(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(30)), silence(secs(1)))
	whole := run(in)

	r := rand.New(rand.NewPCG(9, 10))
	s := New(rate, Defaults())
	var pieces []Phrase
	for rest := in; len(rest) > 0; {
		n := min(r.IntN(5000), len(rest))
		pieces = append(pieces, s.Feed(rest[:n])...)
		rest = rest[n:]
	}
	pieces = append(pieces, s.Flush()...)

	if len(pieces) != len(whole) {
		t.Fatalf("got %d phrases in chunks and %d whole", len(pieces), len(whole))
	}

	for i := range whole {
		if whole[i].Start != pieces[i].Start || whole[i].End != pieces[i].End {
			t.Errorf("phrase %d: %v-%v whole, %v-%v in chunks", i, whole[i].Start, whole[i].End, pieces[i].Start, pieces[i].End)
		}
	}
}

func TestSegmenterCutsALongPhraseThatFollowsAShortOne(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(1)), silence(secs(1)), speech(secs(9.5)), silence(secs(0.1)), speech(secs(5)), silence(secs(1)))

	got := run(in)
	if len(got) != 3 {
		t.Fatalf("got %d phrases, want the short one and the long one cut in two", len(got))
	}

	if cut := got[1].End; cut < secs(12.5) || cut >= secs(12.6) {
		t.Errorf("cut at %v, want inside the short pause at 12.5s to 12.6s", cut)
	}
}

func TestSegmenterKeepsTheSpeechAfterACutThatEndsRightThere(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(9.5)), silence(secs(0.1)), speech(secs(2.2)), silence(secs(1)))

	got := run(in)
	if len(got) != 2 {
		t.Fatalf("got %d phrases, want two", len(got))
	}

	if !near(got[1].End, secs(12.8)+postRoll) {
		t.Errorf("the part after the cut ends at %v, want about %v with all its speech", got[1].End, secs(12.8)+postRoll)
	}
}

// The bug this guards: below 50 Hz a 20 ms frame held no samples at all, and Feed went
// round forever without consuming any, so an odd file hung sublayer listen.
func TestSegmenterFinishesOnARateTooLowForItsFrames(t *testing.T) {
	done := make(chan struct{})
	go func() {
		s := New(10, Defaults())
		s.Feed(make([]float32, 100))
		s.Flush()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Feed at 10 Hz did not return in two seconds")
	}
}

// The bug this guards: one NaN sample made the floor NaN, every comparison with it
// false, and no phrase could open until the NaN left the three-second window.
func TestSegmenterIsNotBlindedByOneNaNSample(t *testing.T) {
	in := signal(silence(secs(1)), speech(secs(1)), silence(secs(1)))
	in[100] = float32(math.NaN())

	if got := run(in); len(got) != 1 {
		t.Errorf("got %d phrases after a NaN sample, want 1", len(got))
	}
}
