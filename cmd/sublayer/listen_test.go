package main

import (
	"testing"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/vad"
)

func TestStampWritesMinutesSecondsAndHundredths(t *testing.T) {
	cases := map[time.Duration]string{
		0:                         "00:00.00",
		1800 * time.Millisecond:   "00:01.80",
		61234 * time.Millisecond:  "01:01.23",
		9999 * time.Millisecond:   "00:10.00",
		754321 * time.Millisecond: "12:34.32",
	}

	for d, want := range cases {
		if got := stamp(d); got != want {
			t.Errorf("stamp(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestDescribeShowsWhereAPhraseStartsEndsAndHowLongItIs(t *testing.T) {
	p := vad.Phrase{Start: 1800 * time.Millisecond, End: 4400 * time.Millisecond}

	if got, want := describe(p), "00:01.80  00:04.40   2.60 s"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
