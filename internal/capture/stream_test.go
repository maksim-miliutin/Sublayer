package capture

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type step struct {
	chunk []float32
	err   error
}

type fakeDevice struct {
	openErr error
	script  []step
	forever []float32

	drains atomic.Int32
	closes atomic.Int32
}

func (f *fakeDevice) open() error {
	return f.openErr
}

func (f *fakeDevice) drain(time.Time) ([]float32, error) {
	n := int(f.drains.Add(1)) - 1
	if n >= len(f.script) {
		return f.forever, nil
	}

	return f.script[n].chunk, f.script[n].err
}

func (f *fakeDevice) close() {
	f.closes.Add(1)
}

func within(t *testing.T, what string, fn func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not finish in two seconds", what)
	}
}

func TestStreamHandsOutChunksInTheirOrder(t *testing.T) {
	d := &fakeDevice{script: []step{{chunk: []float32{1}}, {}, {chunk: []float32{2, 3}}, {}, {chunk: []float32{4}}}}

	source, err := startStream(d, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range [][]float32{{1}, {2, 3}, {4}} {
		got, err := source.Read()
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("read %v, %v; want %v", got, err, want)
		}
	}

	within(t, "Close", func() { source.Close() })

	if _, err := source.Read(); !errors.Is(err, ErrClosed) {
		t.Errorf("read after close gave %v, want ErrClosed", err)
	}
}

func TestStreamReportsWhyTheDeviceWouldNotOpen(t *testing.T) {
	refusal := fmt.Errorf("%w: no endpoint", ErrDevice)
	d := &fakeDevice{openErr: refusal}

	source, err := startStream(d, time.Millisecond)
	if !errors.Is(err, ErrDevice) {
		t.Errorf("got %v, want the refusal", err)
	}

	if source != nil {
		t.Errorf("got a source %v alongside the error", source)
	}

	if n := d.closes.Load(); n != 1 {
		t.Errorf("a device that failed to open was closed %d times, want 1", n)
	}
}

func TestStreamPassesOnADeviceFailureAfterWhatCameBefore(t *testing.T) {
	d := &fakeDevice{script: []step{{chunk: []float32{1}}, {err: fmt.Errorf("%w: unplugged", ErrDevice)}}}

	source, err := startStream(d, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	if got, err := source.Read(); err != nil || !slices.Equal(got, []float32{1}) {
		t.Fatalf("read %v, %v; want the chunk from before the failure", got, err)
	}

	if _, err := source.Read(); !errors.Is(err, ErrDevice) {
		t.Errorf("read gave %v, want the device failure", err)
	}

	within(t, "Close", func() { source.Close() })

	if n := d.closes.Load(); n != 1 {
		t.Errorf("the device was closed %d times, want 1", n)
	}
}

func TestStreamCloseReleasesTheDeviceOnceHoweverOftenCalled(t *testing.T) {
	d := &fakeDevice{}

	source, err := startStream(d, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	within(t, "three Close calls", func() {
		var wg sync.WaitGroup
		for range 3 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				source.Close()
			}()
		}
		wg.Wait()
	})

	if n := d.closes.Load(); n != 1 {
		t.Errorf("the device was closed %d times, want 1", n)
	}
}

func TestStreamCloseWakesAReaderThatIsWaiting(t *testing.T) {
	source, err := startStream(&fakeDevice{}, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan error)
	go func() {
		_, err := source.Read()
		result <- err
	}()

	time.Sleep(20 * time.Millisecond)
	source.Close()

	select {
	case err := <-result:
		if !errors.Is(err, ErrClosed) {
			t.Errorf("the waiting read gave %v, want ErrClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the waiting read never returned")
	}
}

func TestStreamCloseDoesNotHangWhenNobodyReads(t *testing.T) {
	source, err := startStream(&fakeDevice{forever: []float32{1}}, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(150 * time.Millisecond)

	within(t, "Close with a full queue", func() { source.Close() })
}
