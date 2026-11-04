package form

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/maksim-miliutin/Sublayer/internal/settings"
)

var (
	ErrUnsupported = errors.New("form: no settings window on this system yet")
	ErrNumber      = errors.New("form: not a whole number")
	ErrUnknown     = errors.New("form: the settings hold no such field")
)

type Kind int

const (
	Text Kind = iota
	Number
	Choice
	Program
	Model
)

type Option struct {
	Value, Name string
}

type Field struct {
	Label   string
	Kind    Kind
	Value   string
	Options []Option // what a Choice offers
	path    string   // where the settings file keeps the value, as overlay.text_size
}

type Section struct {
	Title  string
	Column int
	Fields []Field
}

type Config struct {
	Settings settings.Settings
	Note     string                // shown beside the buttons
	Save     func([]Section) error // keeps the sections as edited; a failure is told and the window stays
}

var sections = []Section{
	{Title: "Languages", Fields: []Field{
		{Label: "The other side speaks", Kind: Choice, Options: options(settings.Languages), path: "language"},
		{Label: "Translate into", Kind: Choice, Options: options(settings.Targets), path: "target"},
	}},
	{Title: "Recognition: whisper-server", Fields: []Field{
		{Label: "Address", path: "whisper.address"},
		{Label: "Program", Kind: Program, path: "whisper.program"},
		{Label: "Model", Kind: Model, path: "whisper.model"},
	}},
	{Title: "Translation: LibreTranslate", Fields: []Field{
		{Label: "Address", path: "libretranslate.address"},
		{Label: "Program", Kind: Program, path: "libretranslate.program"},
	}},
	{Title: "Keys", Fields: []Field{
		{Label: "Hide subtitles", path: "keys.show"},
		{Label: "Pause", path: "keys.pause"},
		{Label: "Copy the last pair", path: "keys.copy"},
		{Label: "Move subtitles", path: "keys.move"},
	}},
	{Title: "Subtitles", Column: 1, Fields: []Field{
		{Label: "Pairs on screen", Kind: Number, path: "pairs"},
		{Label: "Text size, pt", Kind: Number, path: "overlay.text_size"},
		{Label: "Width, % of the screen", Kind: Number, path: "overlay.width_percent"},
		{Label: "Plate opacity, %", Kind: Number, path: "overlay.background_percent"},
		{Label: "Seconds shown (0: no limit)", Kind: Number, path: "overlay.linger_seconds"},
	}},
	{Title: "Phrases", Column: 1, Fields: []Field{
		{Label: "Shortest speech, ms", Kind: Number, path: "phrases.min_speech_ms"},
		{Label: "Pause ending a phrase, ms", Kind: Number, path: "phrases.end_pause_ms"},
		{Label: "Longest phrase, ms", Kind: Number, path: "phrases.max_phrase_ms"},
	}},
}

func options(codes []string) []Option {
	offered := make([]Option, len(codes))
	for i, code := range codes {
		offered[i] = Option{Value: code, Name: settings.LanguageName(code)}
	}

	return offered
}

func Sections(s settings.Settings) ([]Section, error) {
	root, err := tree(s)
	if err != nil {
		return nil, err
	}

	filled := make([]Section, len(sections))
	for i, section := range sections {
		section.Fields, err = fill(root, section.Fields)
		if err != nil {
			return nil, err
		}
		filled[i] = section
	}

	return filled, nil
}

func fill(root map[string]any, fields []Field) ([]Field, error) {
	filled := slices.Clone(fields)
	for i, f := range filled {
		branch, leaf, err := find(root, f.path)
		if err != nil {
			return nil, err
		}
		filled[i].Value = fmt.Sprint(branch[leaf])
	}

	return filled, nil
}

func Apply(base settings.Settings, edited []Section) (settings.Settings, error) {
	// The fields go into the file form of the settings, so the file's own rules check them
	// and whatever the window leaves out, such as where the subtitles were moved, stays.
	root, err := tree(base)
	if err != nil {
		return settings.Settings{}, err
	}

	for _, f := range flatten(edited) {
		branch, leaf, err := find(root, f.path)
		if err != nil {
			return settings.Settings{}, err
		}

		value, err := convert(f)
		if err != nil {
			return settings.Settings{}, err
		}
		branch[leaf] = value
	}

	file, err := json.Marshal(root)
	if err != nil {
		return settings.Settings{}, err
	}

	return settings.Parse(file)
}

func flatten(sections []Section) []Field {
	var all []Field
	for _, section := range sections {
		all = append(all, section.Fields...)
	}

	return all
}

func convert(f Field) (any, error) {
	text := strings.Trim(strings.TrimSpace(f.Value), `"`)
	if f.Kind != Number {
		return text, nil
	}

	n, err := strconv.Atoi(text)
	if err != nil {
		return nil, fmt.Errorf("%w: %s %q", ErrNumber, f.Label, f.Value)
	}

	return n, nil
}

func tree(s settings.Settings) (map[string]any, error) {
	file, err := settings.Encode(s)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(file))
	decoder.UseNumber()

	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}

	return root, nil
}

func find(root map[string]any, path string) (map[string]any, string, error) {
	names := strings.Split(path, ".")

	branch := root
	for _, name := range names[:len(names)-1] {
		next, nested := branch[name].(map[string]any)
		if !nested {
			return nil, "", fmt.Errorf("%w: %s", ErrUnknown, path)
		}
		branch = next
	}

	leaf := names[len(names)-1]
	if _, held := branch[leaf]; !held {
		return nil, "", fmt.Errorf("%w: %s", ErrUnknown, path)
	}

	return branch, leaf, nil
}
