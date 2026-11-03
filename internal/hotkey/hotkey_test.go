package hotkey

import (
	"errors"
	"testing"
)

func TestParseReadsACombinationWhicheverWayItIsWritten(t *testing.T) {
	cases := map[string]Combo{
		"Ctrl+Alt+T":       {Ctrl: true, Alt: true, Key: "T"},
		"ctrl + alt + t":   {Ctrl: true, Alt: true, Key: "T"},
		"Alt+Control+P":    {Ctrl: true, Alt: true, Key: "P"},
		"Win+Shift+5":      {Win: true, Shift: true, Key: "5"},
		"Ctrl+F12":         {Ctrl: true, Key: "F12"},
		"Shift+Alt+f1":     {Alt: true, Shift: true, Key: "F1"},
		"Ctrl+Shift+Alt+Z": {Ctrl: true, Alt: true, Shift: true, Key: "Z"},
		"Ctrl+Alt+A":       {Ctrl: true, Alt: true, Key: "A"},
		"Ctrl+Alt+9":       {Ctrl: true, Alt: true, Key: "9"},
		"Ctrl+F24":         {Ctrl: true, Key: "F24"},
	}

	for text, want := range cases {
		if got, err := Parse(text); err != nil || got != want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", text, got, err, want)
		}
	}
}

func TestParseRefusesWhatCannotBeAHotkey(t *testing.T) {
	for _, text := range []string{"", "T", "Ctrl+Alt", "Shift+T", "Ctrl+Ctrl+T", "Ctrl+Alt+T+Y", "Ctrl+Alt+Space", "Ctrl+F25", "Ctrl+F0", "Hyper+T", "Ctrl++T", "Ctrl+Alt+Ж"} {
		if got, err := Parse(text); !errors.Is(err, ErrInvalid) {
			t.Errorf("Parse(%q) = %+v, %v; want ErrInvalid", text, got, err)
		}
	}
}

func TestStringWritesModifiersInOneOrder(t *testing.T) {
	c := Combo{Win: true, Shift: true, Alt: true, Ctrl: true, Key: "M"}

	if got := c.String(); got != "Ctrl+Alt+Shift+Win+M" {
		t.Errorf("got %q, want Ctrl+Alt+Shift+Win+M", got)
	}
}

func TestWindowsNumbersOfACombination(t *testing.T) {
	cases := []struct {
		combo     Combo
		modifiers uint32
		key       uint32
	}{
		{Combo{Ctrl: true, Alt: true, Key: "T"}, 0x0003, 0x54},
		{Combo{Shift: true, Win: true, Key: "0"}, 0x000c, 0x30},
		{Combo{Ctrl: true, Key: "F1"}, 0x0002, 0x70},
		{Combo{Alt: true, Key: "F24"}, 0x0001, 0x87},
	}

	for _, c := range cases {
		if m, k := c.combo.Windows(); m != c.modifiers || k != c.key {
			t.Errorf("%s gave %#x and %#x, want %#x and %#x", c.combo, m, k, c.modifiers, c.key)
		}
	}
}
