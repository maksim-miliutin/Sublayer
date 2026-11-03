//go:build !windows

package controls

func Open([]string) (Panel, error) {
	return nil, ErrUnsupported
}
