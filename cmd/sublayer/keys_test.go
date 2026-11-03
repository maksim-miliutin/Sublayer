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
	keys, err := keysFor(settings.Keys{Show: "Ctrl+Alt+T", Pause: "Ctrl+Shift+P", Copy: "Win+C", Move: "Alt+F2"})
	if err != nil {
		t.Fatal(err)
	}

	want := []controls.Key{
		{Combo: hotkey.Combo{Ctrl: true, Alt: true, Key: "T"}, Choice: controls.Choice{Command: controls.ToggleHidden}},
		{Combo: hotkey.Combo{Ctrl: true, Shift: true, Key: "P"}, Choice: controls.Choice{Command: controls.TogglePause}},
		{Combo: hotkey.Combo{Win: true, Key: "C"}, Choice: controls.Choice{Command: controls.CopyLast}},
		{Combo: hotkey.Combo{Alt: true, Key: "F2"}, Choice: controls.Choice{Command: controls.ToggleMove}},
	}
	if !slices.Equal(keys, want) {
		t.Errorf("got %+v, want %+v", keys, want)
	}
}

func TestKeysForRefuseAKeyThatIsNoCombination(t *testing.T) {
	if _, err := keysFor(settings.Keys{Show: "T", Pause: "Ctrl+Alt+P", Copy: "Ctrl+Alt+C", Move: "Ctrl+Alt+M"}); !errors.Is(err, hotkey.ErrInvalid) {
		t.Errorf("got %v, want hotkey.ErrInvalid", err)
	}
}

func TestKeyOfNamesTheKeyOfACommand(t *testing.T) {
	keys, err := keysFor(settings.Defaults().Keys)
	if err != nil {
		t.Fatal(err)
	}

	if got := keyOf(keys, controls.ToggleMove); got != "Ctrl+Alt+M" {
		t.Errorf("got %q, want Ctrl+Alt+M", got)
	}

	if got := keyOf(nil, controls.ToggleMove); got != "" {
		t.Errorf("without keys got %q, want nothing", got)
	}
}
