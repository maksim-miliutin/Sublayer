package main

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
)

type fakeSource struct {
	chunks [][]float32
	end    error
}

func (f *fakeSource) Read() ([]float32, error) {
	if len(f.chunks) == 0 {
		return nil, f.end
	}

	chunk := f.chunks[0]
	f.chunks = f.chunks[1:]

	return chunk, nil
}

func (f *fakeSource) Close() error {
	return nil
}

func TestCollectKeepsEverythingUntilTheSourceCloses(t *testing.T) {
	source := &fakeSource{chunks: [][]float32{{1, 2}, {3}}, end: capture.ErrClosed}

	samples, err := collect(source)
	if err != nil {
		t.Fatalf("a closed source gave %v, want a clean end", err)
	}

	if !slices.Equal(samples, []float32{1, 2, 3}) {
		t.Errorf("got %v, want 1 2 3", samples)
	}
}

func TestCollectHandsBackWhatCameBeforeADeviceFailure(t *testing.T) {
	source := &fakeSource{chunks: [][]float32{{1, 2}, {3}}, end: fmt.Errorf("%w: unplugged", capture.ErrDevice)}

	samples, err := collect(source)
	if !errors.Is(err, capture.ErrDevice) {
		t.Errorf("got %v, want the device failure", err)
	}

	if !slices.Equal(samples, []float32{1, 2, 3}) {
		t.Errorf("got %v, want the 1 2 3 that came before the failure", samples)
	}
}
