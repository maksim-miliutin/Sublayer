package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/controls"
	"github.com/maksim-miliutin/Sublayer/internal/hotkey"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
)

func TestKeysForBindEachSettingToItsChoice(t *testing.T) {
	keys, err := keysFor(settings.Keys{Show: "Ctrl+Alt+T", Pause: "Ctrl+Shift+P", Copy: "Win+C"})
	if err != nil {
		t.Fatal(err)
	}

	want := []controls.Key{
		{Combo: hotkey.Combo{Ctrl: true, Alt: true, Key: "T"}, Choice: controls.Choice{Command: controls.ToggleHidden}},
		{Combo: hotkey.Combo{Ctrl: true, Shift: true, Key: "P"}, Choice: controls.Choice{Command: controls.TogglePause}},
		{Combo: hotkey.Combo{Win: true, Key: "C"}, Choice: controls.Choice{Command: controls.CopyLast}},
	}
	if !slices.Equal(keys, want) {
		t.Errorf("got %+v, want %+v", keys, want)
	}
}

func TestKeysForRefuseAKeyThatIsNoCombination(t *testing.T) {
	if _, err := keysFor(settings.Keys{Show: "T", Pause: "Ctrl+Alt+P", Copy: "Ctrl+Alt+C"}); !errors.Is(err, hotkey.ErrInvalid) {
		t.Errorf("got %v, want hotkey.ErrInvalid", err)
	}
}
