package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

func TestRunRefusesWhatItDoesNotKnow(t *testing.T) {
	refused := [][]string{
		{"a.wav"},
		{"-loud"},
		{"-lang", "de"},
		{"-lang", "fr", "a.wav"},
		{"record"},
		{"record", "a.wav", "b.wav"},
		{"listen", "a.wav", "b.wav"},
		{"listen", "-lang", "de"},
		{"listen", "-lang", "french"},
		{"listen", "-lang"},
		{"listen", "-loud"},
		{"listen", "a.wav", "-lang", "fr"},
		{"overlay", "extra"},
		{"play"},
	}

	for _, args := range refused {
		if err := run(args); !errors.Is(err, errUsage) {
			t.Errorf("%q gave %v, want the usage", args, err)
		}
	}
}

func TestSampleFrameShowsTheLatestPairsOldestFirst(t *testing.T) {
	cases := []struct {
		n    int
		want []ui.Pair
	}{
		{0, samples[:1]},
		{2, samples[:3]},
		{5, samples[2:6]},
		{len(samples), append(slices.Clone(samples[len(samples)-3:]), samples[0])},
	}

	for _, c := range cases {
		if got := sampleFrame(c.n).Pairs; !slices.Equal(got, c.want) {
			t.Errorf("frame %d shows %d pairs %v, want %v", c.n, len(got), got, c.want)
		}
	}
}

func TestSampleFramesWalkThroughEveryState(t *testing.T) {
	seen := map[ui.State]bool{}
	for n := range len(sampleStates) {
		seen[sampleFrame(n).State] = true
	}

	for _, s := range []ui.State{ui.Listening, ui.Recognizing, ui.Paused, ui.Failed} {
		if !seen[s] {
			t.Errorf("state %d never shows up in the samples", s)
		}
	}
}
