package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/maksim-miliutin/Sublayer/internal/servers"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
)

var errUsage = fmt.Errorf("usage: sublayer %[1]s | sublayer record <file.wav> | sublayer listen %[1]s [file.wav] | sublayer overlay", "[-lang "+strings.Join(settings.Languages, "|")+"]")

type machine struct {
	settings func() (settings.Settings, error)
	servers  func(ctx context.Context, s settings.Settings, starting func(servers.Server)) (stop func() error, err error)
}

func main() {
	err := run(os.Args[1:], machine{settings: loadSettings, servers: startServers})
	if err == nil {
		return
	}

	fmt.Fprintln(os.Stderr, err)

	if errors.Is(err, errUsage) {
		os.Exit(2)
	}

	os.Exit(1)
}

func run(args []string, m machine) error {
	switch {
	case len(args) == 0 || strings.HasPrefix(args[0], "-"):
		return subtitles(args, m)
	case len(args) == 2 && args[0] == "record":
		return record(args[1])
	case args[0] == "listen":
		return listen(args[1:], m)
	case len(args) == 1 && args[0] == "overlay":
		return overlay()
	}

	return errUsage
}
