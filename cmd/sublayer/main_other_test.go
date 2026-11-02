//go:build !windows

package main

import (
	"errors"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

// Away from Windows the window and the speakers refuse at once, so a command can be
// followed to the device it opens first without anything being opened.
func TestRunTakesEachCommandToTheDeviceItOpensFirst(t *testing.T) {
	cases := []struct {
		args []string
		want error
	}{
		{nil, ui.ErrUnsupported},
		{[]string{"-lang", "fr"}, ui.ErrUnsupported},
		{[]string{"overlay"}, ui.ErrUnsupported},
		{[]string{"listen"}, capture.ErrUnsupported},
		{[]string{"listen", "-lang", "en"}, capture.ErrUnsupported},
		{[]string{"record", "a.wav"}, capture.ErrUnsupported},
	}

	for _, c := range cases {
		if err := run(c.args, offline); !errors.Is(err, c.want) {
			t.Errorf("%q gave %v, want %v", c.args, err, c.want)
		}
	}
}
