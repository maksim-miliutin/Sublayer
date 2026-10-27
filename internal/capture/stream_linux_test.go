package capture

import (
	"sync"
	"syscall"
	"testing"
	"time"
)

type threadDevice struct {
	fakeDevice

	mu      sync.Mutex
	threads map[int]bool
}

func (d *threadDevice) note() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.threads[syscall.Gettid()] = true
}

func (d *threadDevice) open() error {
	d.note()

	return nil
}

func (d *threadDevice) drain(now time.Time) ([]float32, error) {
	d.note()
	time.Sleep(time.Millisecond)

	return d.fakeDevice.drain(now)
}

func (d *threadDevice) close() {
	d.note()
}

func TestStreamKeepsTheDeviceOnOneThread(t *testing.T) {
	d := &threadDevice{threads: map[int]bool{}}

	source, err := startStream(d, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)
	within(t, "Close", func() { source.Close() })

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.threads) != 1 {
		t.Errorf("the device was used from %d threads, want 1", len(d.threads))
	}
}
