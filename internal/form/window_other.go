//go:build !windows

package form

type Editor struct{}

func (*Editor) Edit(Config) error {
	return ErrUnsupported
}

func (*Editor) Close() {}
