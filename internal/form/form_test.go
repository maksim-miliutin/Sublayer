package form

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/settings"
)

func fields(sections []Section) []*Field {
	var all []*Field
	for i := range sections {
		for j := range sections[i].Fields {
			all = append(all, &sections[i].Fields[j])
		}
	}

	return all
}

func labelled(t *testing.T, sections []Section, label string) *Field {
	t.Helper()

	for _, f := range fields(sections) {
		if f.Label == label {
			return f
		}
	}
	t.Fatalf("no field is labelled %q", label)

	return nil
}

func sectionsOf(t *testing.T, s settings.Settings) []Section {
	t.Helper()

	sections, err := Sections(s)
	if err != nil {
		t.Fatal(err)
	}

	return sections
}

func leaves(tree map[string]any, prefix string) []string {
	var paths []string
	for name, value := range tree {
		branch, nested := value.(map[string]any)
		if !nested {
			paths = append(paths, prefix+name)

			continue
		}
		paths = append(paths, leaves(branch, prefix+name+".")...)
	}

	return paths
}

func TestSectionsShowWhatTheSettingsHold(t *testing.T) {
	s := settings.Defaults()
	s.Whisper.Program = "C:/whisper/Release/whisper-server.exe"
	sections := sectionsOf(t, s)

	cases := map[string]string{
		"The other side speaks": "auto",
		"Translate into":        "ru",
		"Program":               "C:/whisper/Release/whisper-server.exe",
		"Pairs on screen":       "3",
		"Text size, pt":         "12",
		"Longest phrase, ms":    "12000",
		"Hide subtitles":        "Ctrl+Alt+T",
	}
	for label, want := range cases {
		if got := labelled(t, sections, label).Value; got != want {
			t.Errorf("%s shows %q, want %q", label, got, want)
		}
	}
}

func TestSectionsLeaveTheFieldsTheyWereMadeFromAlone(t *testing.T) {
	first := sectionsOf(t, settings.Defaults())

	other := settings.Defaults()
	other.Pairs = 5
	sectionsOf(t, other)

	if got := labelled(t, first, "Pairs on screen").Value; got != "3" {
		t.Errorf("the first sections now show %s pairs, want the 3 they were made with", got)
	}
}

func TestLanguagesAreOfferedByName(t *testing.T) {
	sections := sectionsOf(t, settings.Defaults())

	spoken := []Option{{"auto", "Detect for each phrase"}, {"fr", "French"}, {"en", "English"}}
	if got := labelled(t, sections, "The other side speaks").Options; !slices.Equal(got, spoken) {
		t.Errorf("the other side's languages are %v, want %v", got, spoken)
	}

	targets := []Option{{"ru", "Russian"}, {"en", "English"}}
	if got := labelled(t, sections, "Translate into").Options; !slices.Equal(got, targets) {
		t.Errorf("the translations are %v, want %v", got, targets)
	}
}

func TestTheSectionsCoverEverySettingButTheCorner(t *testing.T) {
	s := settings.Defaults()
	s.Corner = &settings.Corner{X: 40, Y: 980}

	file, err := settings.Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	var tree map[string]any
	if err := json.Unmarshal(file, &tree); err != nil {
		t.Fatal(err)
	}

	got := []string{"corner.x", "corner.y"}
	for _, f := range fields(sectionsOf(t, s)) {
		got = append(got, f.path)
	}

	want := leaves(tree, "")
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("the form covers %v, want every setting once: %v", got, want)
	}
}

func TestApplyGivesBackTheSettingsTheSectionsCameFrom(t *testing.T) {
	s := settings.Defaults()
	s.Language, s.Libre.Program = "fr", "C:/lt-venv/Scripts/libretranslate.exe"
	s.Corner = &settings.Corner{X: 40, Y: 980}

	got, err := Apply(s, sectionsOf(t, s))
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Errorf("got %+v, %v; want the settings back unchanged", got, err)
	}
}

func TestApplyTakesEveryEditedField(t *testing.T) {
	edits := map[string]string{
		"language":                   "en",
		"target":                     "en",
		"whisper.address":            "127.0.0.1:9000",
		"whisper.program":            "D:/w/whisper-server.exe",
		"whisper.model":              "D:/w/ggml-small.bin",
		"libretranslate.address":     "127.0.0.1:5001",
		"libretranslate.program":     "D:/lt/libretranslate.exe",
		"pairs":                      "2",
		"overlay.text_size":          "16",
		"overlay.width_percent":      "40",
		"overlay.background_percent": "50",
		"overlay.linger_seconds":     "0",
		"phrases.min_speech_ms":      "200",
		"phrases.end_pause_ms":       "500",
		"phrases.max_phrase_ms":      "8000",
		"keys.show":                  "Ctrl+Shift+T",
		"keys.pause":                 "Ctrl+Shift+P",
		"keys.copy":                  "Ctrl+Shift+C",
		"keys.move":                  "Ctrl+Shift+M",
	}
	sections := sectionsOf(t, settings.Defaults())
	for _, f := range fields(sections) {
		f.Value = edits[f.path]
	}

	want := settings.Settings{
		Language: "en",
		Target:   "en",
		Whisper:  settings.Whisper{Server: settings.Server{Address: "127.0.0.1:9000", Program: "D:/w/whisper-server.exe"}, Model: "D:/w/ggml-small.bin"},
		Libre:    settings.Server{Address: "127.0.0.1:5001", Program: "D:/lt/libretranslate.exe"},
		Pairs:    2,
		Overlay:  settings.Overlay{TextSize: 16, WidthPercent: 40, BackgroundPercent: 50},
		Phrases:  settings.Phrases{MinSpeechMS: 200, EndPauseMS: 500, MaxPhraseMS: 8000},
		Keys:     settings.Keys{Show: "Ctrl+Shift+T", Pause: "Ctrl+Shift+P", Copy: "Ctrl+Shift+C", Move: "Ctrl+Shift+M"},
	}
	if got, err := Apply(settings.Defaults(), sections); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestApplyLaysTheFieldsOverWhatTheFileHoldsByThen(t *testing.T) {
	sections := sectionsOf(t, settings.Defaults())
	labelled(t, sections, "Text size, pt").Value = "16"

	latest := settings.Defaults()
	latest.Corner = &settings.Corner{X: 40, Y: 980}

	got, err := Apply(latest, sections)
	if err != nil || got.Overlay.TextSize != 16 || got.Corner == nil || *got.Corner != *latest.Corner {
		t.Errorf("got text size %d and corner %+v, %v; want 16 and the corner kept", got.Overlay.TextSize, got.Corner, err)
	}
}

func TestApplyTakesAPathCopiedWithItsQuotes(t *testing.T) {
	sections := sectionsOf(t, settings.Defaults())
	labelled(t, sections, "Program").Value = ` "C:\Program Files\whisper\whisper-server.exe" `

	got, err := Apply(settings.Defaults(), sections)
	if err != nil || got.Whisper.Program != `C:\Program Files\whisper\whisper-server.exe` {
		t.Errorf("got %q, %v; want the path without its quotes and spaces", got.Whisper.Program, err)
	}
}

func TestApplyRefusesAFieldThatIsNoWholeNumber(t *testing.T) {
	for _, value := range []string{"12a", "", "1.5"} {
		sections := sectionsOf(t, settings.Defaults())
		labelled(t, sections, "Text size, pt").Value = value

		_, err := Apply(settings.Defaults(), sections)
		if !errors.Is(err, ErrNumber) || !strings.Contains(err.Error(), "Text size, pt") {
			t.Errorf("%q: got %v, want ErrNumber naming the field", value, err)
		}
	}
}

func TestApplyRefusesWhatTheSettingsRefuse(t *testing.T) {
	small := sectionsOf(t, settings.Defaults())
	labelled(t, small, "Text size, pt").Value = "7"

	twice := sectionsOf(t, settings.Defaults())
	labelled(t, twice, "Pause").Value = "Ctrl+Alt+T"

	for name, sections := range map[string][]Section{"text too small": small, "one key twice": twice} {
		if _, err := Apply(settings.Defaults(), sections); !errors.Is(err, settings.ErrInvalid) {
			t.Errorf("%s: got %v, want settings.ErrInvalid", name, err)
		}
	}
}

func TestApplyRefusesAFieldTheSettingsDoNotHave(t *testing.T) {
	for _, path := range []string{"overlay.speed", "colours.text"} {
		strange := []Section{{Title: "Strange", Fields: []Field{{Label: "Strange", Value: "1", path: path}}}}

		if _, err := Apply(settings.Defaults(), strange); !errors.Is(err, ErrUnknown) {
			t.Errorf("%s: got %v, want ErrUnknown", path, err)
		}
	}
}
