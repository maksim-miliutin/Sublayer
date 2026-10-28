package main

import (
	"errors"
	"io"
	"os"
	"os/signal"

	"github.com/maksim-miliutin/Sublayer/internal/capture"
)

func closeOnInterrupt(c io.Closer) (stop func()) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	finished := make(chan struct{})
	go func() {
		select {
		case <-interrupt:
			c.Close()
		case <-finished:
		}
	}()

	return func() {
		signal.Stop(interrupt)
		close(finished)
	}
}

func unlessClosed(err error) error {
	if errors.Is(err, capture.ErrClosed) {
		return nil
	}

	return err
}
