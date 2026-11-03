package servers

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

var errNoFile = errors.New("exec: file does not exist")

type program struct {
	done    chan struct{}
	output  string
	stopped int
}

func running() *program {
	return &program{done: make(chan struct{})}
}

func (p *program) Done() <-chan struct{} { return p.done }

func (p *program) Output() string { return p.output }

func (p *program) Stop() error {
	p.stopped++

	return nil
}

type machine struct {
	mu        sync.Mutex
	answering map[string]bool
	programs  map[string]*program
	events    []string
	refuse    error
}

func (m *machine) answers(address string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, "ask "+address)

	return m.answering[address]
}

func (m *machine) start(name string, args []string) (Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, "start "+name+" "+strings.Join(args, " "))
	if m.refuse != nil {
		return nil, m.refuse
	}

	p, known := m.programs[name]
	if !known {
		p = running()
	}

	return p, nil
}

func (m *machine) answer(address string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.answering[address] = true
}

func (m *machine) config() Config {
	return Config{Answers: m.answers, Start: m.start, Every: time.Millisecond, Patience: 50 * time.Millisecond}
}

func newMachine() *machine {
	return &machine{answering: map[string]bool{}, programs: map[string]*program{}}
}

var whisper = Server{Name: "whisper-server", Address: "127.0.0.1:8178", Program: "whisper-server.exe", Args: []string{"-m", "small.bin"}}

var libre = Server{Name: "LibreTranslate", Address: "127.0.0.1:5000", Program: "libretranslate.exe"}

func TestStartLeavesAServerThatAlreadyAnswersAlone(t *testing.T) {
	m := newMachine()
	m.answer(whisper.Address)

	r, err := Start(context.Background(), m.config(), whisper)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Stop(); err != nil || slices.ContainsFunc(m.events, func(e string) bool { return strings.HasPrefix(e, "start") }) {
		t.Errorf("events %q, stop %v; want nothing started", m.events, err)
	}
}

func TestStartRunsTheProgramOfAServerThatDoesNotAnswerAndWaitsForIt(t *testing.T) {
	m := newMachine()
	p := running()
	m.programs[whisper.Program] = p

	var told []string
	c := m.config()
	c.Starting = func(s Server) { told = append(told, s.Name) }
	c.Answers = func(address string) bool {
		if n := len(m.events); n > 0 && strings.HasPrefix(m.events[n-1], "start") {
			m.answer(address)
		}

		return m.answers(address)
	}

	r, err := Start(context.Background(), c, whisper)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"ask 127.0.0.1:8178", "start whisper-server.exe -m small.bin", "ask 127.0.0.1:8178"}
	if !slices.Equal(m.events, want) || !slices.Equal(told, []string{"whisper-server"}) {
		t.Errorf("events %q, told of %q; want %q and whisper-server told of", m.events, told, want)
	}

	if r.Stop(); p.stopped != 1 {
		t.Errorf("the program was stopped %d times on Stop, want once", p.stopped)
	}
}

func TestStartRefusesAServerThatNeitherAnswersNorHasAProgram(t *testing.T) {
	m := newMachine()
	manual := libre
	manual.Program = ""

	_, err := Start(context.Background(), m.config(), manual)
	if !errors.Is(err, ErrNotRunning) || !strings.Contains(err.Error(), "LibreTranslate") || !strings.Contains(err.Error(), "127.0.0.1:5000") {
		t.Errorf("got %v, want ErrNotRunning naming LibreTranslate at its address", err)
	}
}

func TestStartPassesOnAProgramThatCannotBeStarted(t *testing.T) {
	m := newMachine()
	m.refuse = errNoFile

	_, err := Start(context.Background(), m.config(), whisper)
	if !errors.Is(err, errNoFile) || !strings.Contains(err.Error(), "whisper-server") {
		t.Errorf("got %v, want the start failure naming whisper-server", err)
	}
}

func TestStartTellsWhatAProgramSaidBeforeItStopped(t *testing.T) {
	m := newMachine()
	p := running()
	p.output = "error: failed to open 'small.bin'"
	close(p.done)
	m.programs[whisper.Program] = p

	_, err := Start(context.Background(), m.config(), whisper)
	if !errors.Is(err, ErrStopped) || !strings.Contains(err.Error(), p.output) {
		t.Errorf("got %v, want ErrStopped with what the program said", err)
	}
}

func TestStartGivesUpOnAProgramThatNeverAnswersAndStopsIt(t *testing.T) {
	m := newMachine()
	p := running()
	m.programs[whisper.Program] = p

	_, err := Start(context.Background(), m.config(), whisper)
	if !errors.Is(err, ErrSlow) || p.stopped != 1 {
		t.Errorf("got %v with the program stopped %d times, want ErrSlow and the program stopped", err, p.stopped)
	}
}

func TestStartGivesUpWhenItIsCancelledAndStopsWhatItStarted(t *testing.T) {
	m := newMachine()
	p := running()
	m.programs[whisper.Program] = p

	c := m.config()
	c.Patience = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	c.Starting = func(Server) { cancel() }

	_, err := Start(ctx, c, whisper)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "whisper-server") || p.stopped != 1 {
		t.Errorf("got %v with the program stopped %d times, want the cancellation and the program stopped", err, p.stopped)
	}
}

func TestStartStartsEveryProgramBeforeWaitingForAny(t *testing.T) {
	m := newMachine()
	c := m.config()
	c.Answers = func(address string) bool {
		started := slices.ContainsFunc(m.events, func(e string) bool { return strings.HasPrefix(e, "start libretranslate") })
		if started {
			m.answer(address)
		}

		return m.answers(address)
	}

	if _, err := Start(context.Background(), c, whisper, libre); err != nil {
		t.Fatal(err)
	}

	startedWhisper := index(m.events, "start whisper-server", 0)
	startedLibre := index(m.events, "start libretranslate", 0)
	waitedForWhisper := index(m.events, "ask "+whisper.Address, startedWhisper+1)
	if startedWhisper < 0 || startedLibre < 0 || waitedForWhisper < 0 || startedLibre > waitedForWhisper {
		t.Errorf("events %q; want LibreTranslate started before whisper-server is waited for", m.events)
	}
}

func index(events []string, prefix string, from int) int {
	for i := max(0, from); i < len(events); i++ {
		if strings.HasPrefix(events[i], prefix) {
			return i
		}
	}

	return -1
}

func TestStartStopsWhatItStartedWhenALaterServerFails(t *testing.T) {
	m := newMachine()
	p := running()
	m.programs[whisper.Program] = p
	manual := libre
	manual.Program = ""

	_, err := Start(context.Background(), m.config(), whisper, manual)
	if !errors.Is(err, ErrNotRunning) || p.stopped != 1 {
		t.Errorf("got %v with whisper-server stopped %d times, want ErrNotRunning and whisper-server stopped", err, p.stopped)
	}
}

func TestStopStopsEachProgramOnceHoweverOftenItIsCalled(t *testing.T) {
	m := newMachine()
	c := m.config()
	c.Answers = func(address string) bool {
		if len(m.events) > 2 {
			m.answer(address)
		}

		return m.answers(address)
	}
	first, second := running(), running()
	m.programs[whisper.Program], m.programs[libre.Program] = first, second

	r, err := Start(context.Background(), c, whisper, libre)
	if err != nil {
		t.Fatal(err)
	}

	r.Stop()
	r.Stop()
	if first.stopped != 1 || second.stopped != 1 {
		t.Errorf("stopped %d and %d times, want each once", first.stopped, second.stopped)
	}
}

func TestTailKeepsTheLastLinesAProgramWrote(t *testing.T) {
	var out tail
	for i := range 300 {
		out.Write([]byte(strings.Repeat("x", 20) + " line " + string(rune('a'+i%26)) + "\n"))
	}
	out.Write([]byte("error: model not found\n\n"))

	got := out.String()
	want := strings.Repeat("x", 20) + " line " + string(rune('a'+296%26))
	if lines := strings.Split(got, "\n"); len(lines) != 5 || lines[0] != want || lines[4] != "error: model not found" {
		t.Errorf("got %q, want the last five lines, from %q to the error", got, want)
	}

	if len(out.text) > kept {
		t.Errorf("kept %d bytes, want at most %d", len(out.text), kept)
	}
}

func TestTailDropsTheCarriageReturnsOfWindowsLines(t *testing.T) {
	var out tail
	out.Write([]byte("loading model\r\nerror: failed to open 'small.bin'\r\n"))

	if got := out.String(); got != "loading model\nerror: failed to open 'small.bin'" {
		t.Errorf("got %q, want the two lines without carriage returns", got)
	}
}

func TestTailTakesWritesFromSeveralGoroutines(t *testing.T) {
	var out tail
	var wrote sync.WaitGroup
	for range 4 {
		wrote.Add(1)
		go func() {
			defer wrote.Done()

			for range 100 {
				out.Write([]byte("line\n"))
			}
		}()
	}
	wrote.Wait()

	if got := out.String(); !strings.HasSuffix(got, "line") {
		t.Errorf("got %q, want lines", got)
	}
}
