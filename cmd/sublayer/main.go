package main

import (
	"errors"
	"fmt"
	"os"
)

var errUsage = errors.New("usage: sublayer record <file.wav> | sublayer overlay")

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
	case len(args) == 2 && args[0] == "record":
		return record(args[1])
	case len(args) == 1 && args[0] == "overlay":
		return overlay()
	}

	return errUsage
}
