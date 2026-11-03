//go:build !windows

package ui

func Open(*Point) (Window, error) {
	return nil, ErrUnsupported
}
