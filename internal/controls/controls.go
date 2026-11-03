package controls

import (
	"errors"

	"github.com/maksim-miliutin/Sublayer/internal/hotkey"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

var (
	ErrUnsupported = errors.New("controls: no tray icon on this system yet")
	ErrTray        = errors.New("controls: the tray icon failed")
)

type Command int

const (
	TogglePause Command = iota
	ToggleHidden
	ChooseLanguage
	CopyLast
	ToggleMove
	Quit
)

type Choice struct {
	Command  Command
	Language string // the one chosen by ChooseLanguage
}

type Key struct {
	Combo  hotkey.Combo
	Choice Choice
}

type Config struct {
	Languages []string
	Keys      []Key
}

type Status struct {
	State    ui.State
	Language string
	Target   string
	Paused   bool
	Hidden   bool
	Moving   bool
	Starting string // the server Sublayer is starting; empty once it listens
}

type Panel interface {
	Show(Status) error
	Alert(text string) error
	Copy(text string) error
	Choices() <-chan Choice
	Close() error
}
