package controls

import "github.com/maksim-miliutin/Sublayer/internal/ui"

type Command int

const (
	TogglePause Command = iota
	ToggleHidden
	ChooseLanguage
	Quit
)

type Choice struct {
	Command  Command
	Language string // the one chosen by ChooseLanguage
}

type Status struct {
	State    ui.State
	Language string
	Target   string
	Paused   bool
	Hidden   bool
	Starting string // the server Sublayer is starting; empty once it listens
}

type Panel interface {
	Show(Status) error
	Alert(text string) error
	Choices() <-chan Choice
	Close() error
}
