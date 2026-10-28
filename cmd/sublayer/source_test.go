package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
)

func TestUnlessClosedTurnsAClosedSourceIntoACleanEnd(t *testing.T) {
	for _, err := range []error{capture.ErrClosed, fmt.Errorf("stopping: %w", capture.ErrClosed)} {
		if got := unlessClosed(err); got != nil {
			t.Errorf("unlessClosed(%v) = %v, want nil", err, got)
		}
	}
}

func TestUnlessClosedPassesEveryOtherFailureOn(t *testing.T) {
	failure := fmt.Errorf("%w: unplugged", capture.ErrDevice)

	if got := unlessClosed(failure); !errors.Is(got, capture.ErrDevice) {
		t.Errorf("got %v, want the device failure", got)
	}
}
