package controls

import (
	"unicode/utf16"

	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

const tipLimit = 127 // characters the tray shows of a tooltip

var names = map[string]string{"auto": "Detect for each phrase", "fr": "French", "en": "English"}

type item struct {
	id      int // what the menu answers when the item is clicked; zero for a line or a title
	label   string
	choice  Choice
	checked bool
	title   bool
	line    bool
}

func menu(s Status, languages []string) []item {
	items := []item{{label: "The other side speaks", title: true}}
	for _, code := range languages {
		items = append(items, item{label: name(code), choice: Choice{Command: ChooseLanguage, Language: code}, checked: code == s.Language})
	}

	items = append(items,
		item{line: true},
		item{label: "Pause", choice: Choice{Command: TogglePause}, checked: s.Paused},
		item{label: "Hide subtitles", choice: Choice{Command: ToggleHidden}, checked: s.Hidden},
		item{line: true},
		item{label: "Exit", choice: Choice{Command: Quit}},
	)

	for i := range items {
		if !items[i].line && !items[i].title {
			items[i].id = i + 1
		}
	}

	return items
}

func name(code string) string {
	if n, known := names[code]; known {
		return n
	}

	return code
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

	runes := []rune(text)

	return string(runes[:min(len(runes), tipLimit)])
}

func fill(field []uint16, text string) {
	n := copy(field[:len(field)-1], utf16.Encode([]rune(text)))
	field[n] = 0
}
