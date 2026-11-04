package controls

import (
	"strings"
	"unicode/utf16"

	"github.com/maksim-miliutin/Sublayer/internal/hotkey"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

const tipLimit = 127 // characters the tray shows of a tooltip

type item struct {
	id      int // what the menu answers when the item is clicked; zero for a line or a title
	label   string
	choice  Choice
	checked bool
	title   bool
	line    bool
}

func menu(s Status, c Config) []item {
	items := []item{{label: "The other side speaks", title: true}}
	for _, code := range c.Languages {
		items = append(items, item{label: settings.LanguageName(code), choice: Choice{Command: ChooseLanguage, Language: code}, checked: code == s.Language})
	}

	items = append(items,
		item{line: true},
		item{label: "Pause", choice: Choice{Command: TogglePause}, checked: s.Paused},
		item{label: "Hide subtitles", choice: Choice{Command: ToggleHidden}, checked: s.Hidden},
		item{label: "Copy the last pair", choice: Choice{Command: CopyLast}},
		item{label: "Move subtitles", choice: Choice{Command: ToggleMove}, checked: s.Moving},
		item{line: true},
		item{label: "Settings…", choice: Choice{Command: OpenSettings}},
		item{label: "Exit", choice: Choice{Command: Quit}},
	)

	for i := range items {
		if !items[i].line && !items[i].title {
			items[i].id = i + 1
			items[i].label += shortcut(c.Keys, items[i].choice)
		}
	}

	return items
}

// A menu shows the text after a tab apart, at its right edge.
func shortcut(keys []Key, c Choice) string {
	for _, k := range keys {
		if k.Choice == c {
			return "\t" + k.Combo.String()
		}
	}

	return ""
}

func taken(combos []hotkey.Combo) string {
	names := make([]string, len(combos))
	for i, combo := range combos {
		names[i] = combo.String()
	}

	if len(names) == 1 {
		return names[0] + " is taken by another program"
	}

	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " are taken by another program"
}

func clipText(text string) []uint16 {
	return append(utf16.Encode([]rune(strings.ReplaceAll(text, "\n", "\r\n"))), 0)
}

func tip(s Status) string {
	if s.Language == "" {
		return "Sublayer"
	}

	text := "Sublayer: " + s.Language + " → " + s.Target
	switch {
	case s.Starting != "":
		text += ", starting " + s.Starting
	case s.Paused:
		text += ", paused"
	case s.State == ui.Failed:
		text += ", the last phrase failed"
	}

	if s.Hidden {
		text += ", hidden"
	}

	if s.Moving {
		text += ", moving"
	}

	runes := []rune(text)

	return string(runes[:min(len(runes), tipLimit)])
}

func fill(field []uint16, text string) {
	n := copy(field[:len(field)-1], utf16.Encode([]rune(text)))
	field[n] = 0
}
