package capture

import (
	"runtime"
	"sync"
	"time"
)

const (
	pollEvery = 10 * time.Millisecond
	queued    = 64
)

type device interface {
	open() error
	drain(now time.Time) ([]float32, error)
	close()
}

type stream struct {
	chunks chan []float32
	stop   chan struct{}
	done   chan struct{}
	once   sync.Once
	err    error
}

func startStream(d device, every time.Duration) (Source, error) {
	s := &stream{
		chunks: make(chan []float32, queued),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}

	ready := make(chan error)
	go s.run(d, every, ready)

	if err := <-ready; err != nil {
		return nil, err
	}

	return s, nil
}

// COM is set up and torn down per thread, so the device is opened, polled and
// closed from one thread the goroutine never leaves.
func (s *stream) run(d device, every time.Duration, ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := d.open(); err != nil {
		d.close()
		ready <- err
		return
	}

	ready <- nil

	s.err = s.pump(d, every)
	d.close()
	close(s.chunks)
	close(s.done)
}

func (s *stream) pump(d device, every time.Duration) error {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		var now time.Time
		select {
		case <-s.stop:
			return ErrClosed
		case now = <-ticker.C:
		}

		chunk, err := d.drain(now)
		if err != nil {
			return err
		}

		if len(chunk) == 0 {
			continue
		}

		select {
		case s.chunks <- chunk:
		case <-s.stop:
			return ErrClosed
		}
	}
}

func (s *stream) Read() ([]float32, error) {
	chunk, ok := <-s.chunks
	if !ok {
		return nil, s.err
	}

	return chunk, nil
}

func (s *stream) Close() error {
	s.once.Do(func() { close(s.stop) })
	<-s.done

	return nil
}
