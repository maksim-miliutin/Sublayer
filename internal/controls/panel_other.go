//go:build !windows

package controls

func Open(Config) (Panel, error) {
	return nil, ErrUnsupported
}
