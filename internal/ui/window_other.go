//go:build !windows

package ui

func Open() (Window, error) {
	return nil, ErrUnsupported
}
