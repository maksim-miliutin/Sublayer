package main

import (
	"context"
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
			// The first Ctrl+C lets the command finish what it has; the second is left to
			// end the program the usual way, should finishing hang.
			signal.Stop(interrupt)
			c.Close()
		case <-finished:
		}
	}()

	return func() {
		signal.Stop(interrupt)
		close(finished)
	}
}

type quitter func()

func (q quitter) Close() error {
	q()

	return nil
}

func unlessQuit(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return err
}

func unlessClosed(err error) error {
	if errors.Is(err, capture.ErrClosed) {
		return nil
	}

	return err
}
