package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var errUsage = fmt.Errorf("usage: sublayer %[1]s | sublayer record <file.wav> | sublayer listen %[1]s [file.wav] | sublayer overlay", "[-lang "+strings.Join(languages, "|")+"]")

func main() {
	err := run(os.Args[1:])
	if err == nil {
		return
	}

	fmt.Fprintln(os.Stderr, err)

	if errors.Is(err, errUsage) {
		os.Exit(2)
	}

	os.Exit(1)
}

func run(args []string) error {
	switch {
	case len(args) == 0 || strings.HasPrefix(args[0], "-"):
		return subtitles(args)
	case len(args) == 2 && args[0] == "record":
		return record(args[1])
	case args[0] == "listen":
		return listen(args[1:])
	case len(args) == 1 && args[0] == "overlay":
		return overlay()
	}

	return errUsage
}
