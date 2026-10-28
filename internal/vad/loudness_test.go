package vad

import (
	"math"
	"testing"
)

func sine(n int, amplitude float64) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(amplitude * math.Sin(2*math.Pi*float64(i)/32))
	}

	return out
}

func TestLoudnessOfAFullScaleSineIsMinusThreeDecibels(t *testing.T) {
	if got := loudness(sine(320, 1)); math.Abs(got-(-3.0103)) > 0.01 {
		t.Errorf("got %.4f dB, want -3.01", got)
	}
}

func TestLoudnessDropsSixDecibelsForHalfTheAmplitude(t *testing.T) {
	full, half := loudness(sine(320, 0.5)), loudness(sine(320, 0.25))

	if math.Abs(full-half-6.0206) > 0.01 {
		t.Errorf("halving the amplitude went from %.3f to %.3f dB, want 6.02 dB less", full, half)
	}
}

func TestLoudnessOfSilenceBottomsOut(t *testing.T) {
	if got := loudness(make([]float32, 320)); got != silenceDB {
		t.Errorf("got %v dB for zeros, want %v", got, silenceDB)
	}
}

func TestFloorIsTheQuietestRecentLevel(t *testing.T) {
	f := newFloor(4)
	for _, level := range []float64{-20, -45, -30} {
		f.add(level)
	}

	if got := f.level(); got != -45 {
		t.Errorf("got %v, want the quietest -45", got)
	}
}

func TestFloorForgetsLevelsOlderThanItsSpan(t *testing.T) {
	f := newFloor(3)
	for _, level := range []float64{-60, -20, -25, -30} {
		f.add(level)
	}

	if got := f.level(); got != -30 {
		t.Errorf("got %v, want -30 once -60 has left the span", got)
	}
}

func TestFloorOfNothingYetIsSilence(t *testing.T) {
	if got := newFloor(5).level(); got != silenceDB {
		t.Errorf("got %v, want %v", got, silenceDB)
	}
}
