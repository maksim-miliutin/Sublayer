package hotkey

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalid = errors.New("hotkey: not a key combination")

const lastFunctionKey = 24

type Combo struct {
	Ctrl, Alt, Shift, Win bool
	Key                   string // a letter, a digit or F1 to F24
}

func Parse(text string) (Combo, error) {
	parts := strings.Split(text, "+")

	var c Combo
	for _, part := range parts[:len(parts)-1] {
		held, ok := hold(c, strings.ToLower(strings.TrimSpace(part)))
		if !ok {
			return Combo{}, fmt.Errorf("%w: %q", ErrInvalid, text)
		}
		c = held
	}

	key, ok := canonical(strings.ToUpper(strings.TrimSpace(parts[len(parts)-1])))
	if !ok || !c.Ctrl && !c.Alt && !c.Win {
		return Combo{}, fmt.Errorf("%w: %q, want Ctrl, Alt or Win with one letter, digit or F1 to F24", ErrInvalid, text)
	}
	c.Key = key

	return c, nil
}

func (c Combo) String() string {
	var parts []string
	for _, m := range []struct {
		held bool
		name string
	}{{c.Ctrl, "Ctrl"}, {c.Alt, "Alt"}, {c.Shift, "Shift"}, {c.Win, "Win"}} {
		if m.held {
			parts = append(parts, m.name)
		}
	}

	return strings.Join(append(parts, c.Key), "+")
}

func (c Combo) Windows() (modifiers, key uint32) {
	// RegisterHotKey numbers Alt 1, Ctrl 2, Shift 4 and Win 8; a letter or a digit is its
	// own ASCII code, and F1 is 0x70.
	for bit, held := range []bool{c.Alt, c.Ctrl, c.Shift, c.Win} {
		if held {
			modifiers |= 1 << bit
		}
	}

	if len(c.Key) == 1 {
		return modifiers, uint32(c.Key[0])
	}

	n, err := strconv.Atoi(strings.TrimPrefix(c.Key, "F"))
	if err != nil {
		return modifiers, 0
	}

	return modifiers, 0x6f + uint32(n)
}

func hold(c Combo, modifier string) (Combo, bool) {
	switch {
	case (modifier == "ctrl" || modifier == "control") && !c.Ctrl:
		c.Ctrl = true
	case modifier == "alt" && !c.Alt:
		c.Alt = true
	case modifier == "shift" && !c.Shift:
		c.Shift = true
	case modifier == "win" && !c.Win:
		c.Win = true
	default:
		return Combo{}, false
	}

	return c, true
}

func canonical(key string) (string, bool) {
	if len(key) == 1 {
		return key, key >= "A" && key <= "Z" || key >= "0" && key <= "9"
	}

	n, err := strconv.Atoi(strings.TrimPrefix(key, "F"))
	if !strings.HasPrefix(key, "F") || err != nil || n < 1 || n > lastFunctionKey {
		return "", false
	}

	return "F" + strconv.Itoa(n), true
}
