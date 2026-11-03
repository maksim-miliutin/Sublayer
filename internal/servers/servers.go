package servers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotRunning = errors.New("servers: nothing answers and no program is set to start it")
	ErrStopped    = errors.New("servers: the program stopped before it answered")
	ErrSlow       = errors.New("servers: the program did not answer in time")
)

const (
	kept       = 4096 // bytes of a program's output kept to tell why it stopped
	shownLines = 5
)

type Server struct {
	Name    string
	Address string // host:port
	Program string // empty leaves starting it to you
	Args    []string
}

type Process interface {
	Done() <-chan struct{}
	Output() string
	Stop() error
}

type Config struct {
	Answers  func(address string) bool
	Start    func(program string, args []string) (Process, error)
	Starting func(Server) // told as each program starts; nil when nobody asks
	Every    time.Duration
	Patience time.Duration // how long a started program may take to answer
}

type Running struct {
	started []Process
}

type launched struct {
	server  Server
	process Process
}

func Start(ctx context.Context, c Config, all ...Server) (*Running, error) {
	if c.Starting == nil {
		c.Starting = func(Server) {}
	}

	r := &Running{}
	var waiting []launched
	for _, s := range all {
		if c.Answers(s.Address) {
			continue
		}

		p, err := launch(c, s)
		if err != nil {
			return nil, errors.Join(err, r.Stop())
		}

		r.started = append(r.started, p)
		waiting = append(waiting, launched{server: s, process: p})
	}

	for _, l := range waiting {
		if err := await(ctx, c, l); err != nil {
			return nil, errors.Join(err, r.Stop())
		}
	}

	return r, nil
}

func (r *Running) Stop() error {
	var failed []error
	for i := len(r.started) - 1; i >= 0; i-- {
		failed = append(failed, r.started[i].Stop())
	}
	r.started = nil

	return errors.Join(failed...)
}

func launch(c Config, s Server) (Process, error) {
	if s.Program == "" {
		return nil, fmt.Errorf("%w: %s at %s", ErrNotRunning, s.Name, s.Address)
	}

	p, err := c.Start(s.Program, s.Args)
	if err != nil {
		return nil, fmt.Errorf("servers: %s: %w", s.Name, err)
	}

	c.Starting(s)

	return p, nil
}

func await(ctx context.Context, c Config, l launched) error {
	patience := time.NewTimer(c.Patience)
	defer patience.Stop()

	every := time.NewTicker(c.Every)
	defer every.Stop()

	for !c.Answers(l.server.Address) {
		select {
		case <-l.process.Done():
			return fmt.Errorf("%w: %s:\n%s", ErrStopped, l.server.Name, l.process.Output())
		case <-patience.C:
			return fmt.Errorf("%w: %s at %s within %v", ErrSlow, l.server.Name, l.server.Address, c.Patience)
		case <-ctx.Done():
			return fmt.Errorf("servers: stopped waiting for %s: %w", l.server.Name, ctx.Err())
		case <-every.C:
		}
	}

	return nil
}

type tail struct {
	mu   sync.Mutex
	text []byte
}

func (t *tail) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.text = append(t.text, b...)
	if over := len(t.text) - kept; over > 0 {
		t.text = append(t.text[:0], t.text[over:]...)
	}

	return len(b), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(t.text), "\r", "")), "\n")

	return strings.Join(lines[max(0, len(lines)-shownLines):], "\n")
}
