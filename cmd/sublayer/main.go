package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "record" {
		fmt.Fprintln(os.Stderr, "usage: sublayer record <file.wav>")
		os.Exit(2)
	}

	if err := record(os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
