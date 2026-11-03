//go:build !windows

package ui

func Open(Options) (Window, error) {
	return nil, ErrUnsupported
}
