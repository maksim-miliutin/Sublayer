//go:build !windows || !(amd64 || arm64)

package capture

func Open() (Source, error) {
	return nil, ErrUnsupported
}
