package controls

import (
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

var languages = []string{"auto", "fr", "en"}

func chosen(items []item, c Choice) (item, bool) {
	for _, it := range items {
		if it.id != 0 && it.choice == c {
			return it, true
		}
	}

	return item{}, false
}

func TestMenuOffersEveryLanguageAndTicksTheOneSpoken(t *testing.T) {
	items := menu(Status{Language: "fr", Target: "ru"}, languages)

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

func TestMenuTicksPauseAndHidingAsTheyStand(t *testing.T) {
	for _, s := range []Status{{Paused: true}, {Hidden: true}, {}} {
		items := menu(s, languages)

		pause, _ := chosen(items, Choice{Command: TogglePause})
		hide, _ := chosen(items, Choice{Command: ToggleHidden})
		if pause.checked != s.Paused || hide.checked != s.Hidden {
			t.Errorf("for %+v pause is ticked %v and hiding %v", s, pause.checked, hide.checked)
		}
	}
}

func TestMenuEndsWithExit(t *testing.T) {
	items := menu(Status{}, languages)

	if last := items[len(items)-1]; last.choice != (Choice{Command: Quit}) || last.id == 0 {
		t.Errorf("the menu ends with %+v, want Exit", last)
	}
}

func TestMenuNumbersEveryChoiceApartAndNothingElse(t *testing.T) {
	seen := map[int]bool{}
	for _, it := range menu(Status{}, languages) {
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

func TestALanguageWithoutANameShowsItsCode(t *testing.T) {
	if got := name("de"); got != "de" {
		t.Errorf("de is labelled %q, want de", got)
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
