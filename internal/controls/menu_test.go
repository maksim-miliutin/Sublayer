package controls

import (
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/maksim-miliutin/Sublayer/internal/hotkey"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

var languages = []string{"auto", "fr", "en"}

var keyed = Config{
	Languages: languages,
	Keys: []Key{
		{Combo: hotkey.Combo{Ctrl: true, Alt: true, Key: "T"}, Choice: Choice{Command: ToggleHidden}},
		{Combo: hotkey.Combo{Ctrl: true, Alt: true, Key: "P"}, Choice: Choice{Command: TogglePause}},
		{Combo: hotkey.Combo{Ctrl: true, Alt: true, Key: "C"}, Choice: Choice{Command: CopyLast}},
		{Combo: hotkey.Combo{Ctrl: true, Alt: true, Key: "M"}, Choice: Choice{Command: ToggleMove}},
	},
}

func chosen(items []item, c Choice) (item, bool) {
	for _, it := range items {
		if it.id != 0 && it.choice == c {
			return it, true
		}
	}

	return item{}, false
}

func TestMenuOffersEveryLanguageAndTicksTheOneSpoken(t *testing.T) {
	items := menu(Status{Language: "fr", Target: "ru"}, Config{Languages: languages})

	for _, code := range languages {
		it, found := chosen(items, Choice{Command: ChooseLanguage, Language: code})
		if !found || it.checked != (code == "fr") {
			t.Errorf("%s: found %v, ticked %v; want it offered and ticked only for fr", code, found, it.checked)
		}
	}

	if it, _ := chosen(items, Choice{Command: ChooseLanguage, Language: "en"}); it.label != "English" {
		t.Errorf("en is labelled %q, want English", it.label)
	}
}

func TestMenuTicksPauseHidingAndMovingAsTheyStand(t *testing.T) {
	for _, s := range []Status{{Paused: true}, {Hidden: true}, {Moving: true}, {}} {
		items := menu(s, Config{Languages: languages})

		pause, _ := chosen(items, Choice{Command: TogglePause})
		hide, _ := chosen(items, Choice{Command: ToggleHidden})
		move, _ := chosen(items, Choice{Command: ToggleMove})
		if pause.checked != s.Paused || hide.checked != s.Hidden || move.checked != s.Moving {
			t.Errorf("for %+v pause is ticked %v and hiding %v", s, pause.checked, hide.checked)
		}
	}
}

func TestMenuNamesTheKeyOfEachChoiceThatHasOne(t *testing.T) {
	items := menu(Status{}, keyed)

	cases := map[Command]string{
		TogglePause:  "Pause\tCtrl+Alt+P",
		ToggleHidden: "Hide subtitles\tCtrl+Alt+T",
		CopyLast:     "Copy the last pair\tCtrl+Alt+C",
		ToggleMove:   "Move subtitles\tCtrl+Alt+M",
		OpenSettings: "Settings…",
		Quit:         "Exit",
	}
	for command, want := range cases {
		if it, found := chosen(items, Choice{Command: command}); !found || it.label != want {
			t.Errorf("command %d is labelled %q (found %v), want %q", command, it.label, found, want)
		}
	}
}

func TestClipTextEndsLinesTheWindowsWayAndClosesWithZero(t *testing.T) {
	got := clipText("Oui.\nДа.")
	want := append(utf16.Encode([]rune("Oui.\r\nДа.")), 0)

	if string(utf16.Decode(got)) != string(utf16.Decode(want)) || got[len(got)-1] != 0 {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTakenNamesEveryKeyAnotherProgramHolds(t *testing.T) {
	one := []hotkey.Combo{{Ctrl: true, Alt: true, Key: "T"}}
	two := append(one, hotkey.Combo{Ctrl: true, Alt: true, Key: "C"})

	if got := taken(one); got != "Ctrl+Alt+T is taken by another program" {
		t.Errorf("got %q", got)
	}

	if got := taken(two); got != "Ctrl+Alt+T and Ctrl+Alt+C are taken by another program" {
		t.Errorf("got %q", got)
	}
}

func TestMenuEndsWithExit(t *testing.T) {
	items := menu(Status{}, Config{Languages: languages})

	if last := items[len(items)-1]; last.choice != (Choice{Command: Quit}) || last.id == 0 {
		t.Errorf("the menu ends with %+v, want Exit", last)
	}
}

func TestMenuNumbersEveryChoiceApartAndNothingElse(t *testing.T) {
	seen := map[int]bool{}
	for _, it := range menu(Status{}, keyed) {
		answers := !it.line && !it.title
		if answers == (it.id == 0) || seen[it.id] && it.id != 0 {
			t.Errorf("%+v: a choice wants its own number above zero, a line or a title wants none", it)
		}
		seen[it.id] = true
	}
}

func TestTipNamesTheLanguagesAndWhatIsGoingOn(t *testing.T) {
	cases := []struct {
		status Status
		want   string
	}{
		{Status{}, "Sublayer"},
		{Status{Language: "fr", Target: "ru"}, "Sublayer: fr → ru"},
		{Status{Language: "auto", Target: "ru", Starting: "LibreTranslate"}, "Sublayer: auto → ru, starting LibreTranslate"},
		{Status{Language: "en", Target: "ru", Paused: true, Hidden: true}, "Sublayer: en → ru, paused, hidden"},
		{Status{Language: "fr", Target: "ru", State: ui.Failed}, "Sublayer: fr → ru, the last phrase failed"},
		{Status{Language: "fr", Target: "ru", Moving: true}, "Sublayer: fr → ru, moving"},
	}

	for _, c := range cases {
		if got := tip(c.status); got != c.want {
			t.Errorf("tip(%+v) = %q, want %q", c.status, got, c.want)
		}
	}
}

func TestTipFitsWhatTheTrayShows(t *testing.T) {
	got := tip(Status{Language: "fr", Target: "ru", Starting: strings.Repeat("whisper-server ", 20)})

	if n := len(utf16.Encode([]rune(got))); n > 127 {
		t.Errorf("the tip runs to %d characters, want at most 127", n)
	}
}

func TestFillCutsATextToItsFieldAndEndsIt(t *testing.T) {
	field := []uint16{9, 9, 9, 9, 9, 9, 9, 9}
	fill(field, "Sublayer: fr → ru")

	if got := string(utf16.Decode(field[:7])); got != "Sublaye" || field[7] != 0 {
		t.Errorf("got %q with %d last, want Sublaye and a closing zero", got, field[7])
	}

	fill(field, "fr")
	if got := string(utf16.Decode(field[:2])); got != "fr" || field[2] != 0 {
		t.Errorf("got %q with %d after it, want fr ended by a zero", got, field[2])
	}
}
