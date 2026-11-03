package servers

import (
	"os/exec"
	"time"
)

const outputDrain = 2 * time.Second // a killed program's children may hold its output open

type process struct {
	output *tail
	done   chan struct{}
	kill   func() error
}

func Launch(program string, args []string) (Process, error) {
	output := &tail{}
	cmd := exec.Command(program, args...)
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.WaitDelay = outputDrain
	quiet(cmd)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	kill, err := contain(cmd)
	if err != nil {
		return nil, err
	}

	p := &process{output: output, done: make(chan struct{}), kill: kill}
	go func() {
		cmd.Wait()
		close(p.done)
	}()

	return p, nil
}

func (p *process) Done() <-chan struct{} {
	return p.done
}

func (p *process) Output() string {
	return p.output.String()
}

func (p *process) Stop() error {
	err := p.kill()
	<-p.done

	return err
}
