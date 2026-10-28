package vad

import (
	"math"
	"slices"
)

const silenceDB = -70.0 // decibels below full scale; anything quieter counts as this

func loudness(frame []float32) float64 {
	var sum float64
	for _, s := range frame {
		if !math.IsNaN(float64(s)) {
			sum += float64(s) * float64(s)
		}
	}

	return max(silenceDB, 10*math.Log10(sum/float64(len(frame))+1e-12))
}

type floor struct {
	levels []float64
	next   int
}

func newFloor(frames int) *floor {
	levels := make([]float64, frames)
	for i := range levels {
		levels[i] = math.Inf(1)
	}

	return &floor{levels: levels}
}

func (f *floor) add(level float64) {
	f.levels[f.next] = level
	f.next = (f.next + 1) % len(f.levels)
}

func (f *floor) level() float64 {
	quietest := slices.Min(f.levels)
	if math.IsInf(quietest, 1) {
		return silenceDB
	}

	return quietest
}
