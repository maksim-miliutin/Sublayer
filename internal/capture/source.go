package capture

import "errors"

var (
	ErrUnsupported = errors.New("capture: no loopback capture on this system yet")
	ErrDevice      = errors.New("capture: the audio device failed")
	ErrClosed      = errors.New("capture: the source is closed")
)

const Rate = 16000 // samples per second of every chunk a Source hands out

type Source interface {
	Read() ([]float32, error)
	Close() error
}
