package ui

import (
	"errors"
	"slices"
)

var (
	ErrUnsupported = errors.New("ui: no overlay window on this system yet")
	ErrWindow      = errors.New("ui: the overlay window failed")
)

type Pair struct {
	Original    string
	Translation string
}

type State int

const (
	Listening State = iota
	Recognizing
	Paused
	Failed
)

type Frame struct {
	Pairs  []Pair // oldest first; the newest is shown at the bottom
	State  State
	Hidden bool
	Moving bool
}

func (f Frame) clone() Frame {
	f.Pairs = slices.Clone(f.Pairs)

	return f
}

type Look struct {
	TextPoints        int // the translation's size; the original is drawn three quarters of it
	WidthPercent      int // of the screen's working area
	BackgroundPercent int // how opaque the plates are
}

type Options struct {
	Corner *Point // nil puts the subtitles at the bottom left of the screen
	Look   Look
}

type Point struct {
	X, Y int // screen pixels
}

type Window interface {
	Show(Frame) error
	Style(Look) error
	Corner() Point
	Close() error
}
