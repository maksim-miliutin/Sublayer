package ui

import "errors"

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
	Pairs []Pair // oldest first; the newest is shown at the bottom
	State State
}

type Window interface {
	Show(Frame) error
	Close() error
}
