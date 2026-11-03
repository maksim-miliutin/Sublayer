package main

import (
	"github.com/maksim-miliutin/Sublayer/internal/controls"
	"github.com/maksim-miliutin/Sublayer/internal/hotkey"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
)

func keysFor(k settings.Keys) ([]controls.Key, error) {
	bound := []struct {
		text    string
		command controls.Command
	}{{k.Show, controls.ToggleHidden}, {k.Pause, controls.TogglePause}, {k.Copy, controls.CopyLast}, {k.Move, controls.ToggleMove}}

	keys := make([]controls.Key, 0, len(bound))
	for _, b := range bound {
		combo, err := hotkey.Parse(b.text)
		if err != nil {
			return nil, err
		}

		keys = append(keys, controls.Key{Combo: combo, Choice: controls.Choice{Command: b.command}})
	}

	return keys, nil
}

func keyOf(keys []controls.Key, command controls.Command) string {
	for _, k := range keys {
		if k.Choice.Command == command {
			return k.Combo.String()
		}
	}

	return ""
}
