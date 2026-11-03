//go:build windows && (amd64 || arm64)

package servers

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/maksim-miliutin/Sublayer/internal/winapi"
)

var ErrJob = errors.New("servers: Windows did not tie the program to Sublayer")

const (
	createNoWindow    = 0x08000000
	extendedLimitInfo = 9
	killOnJobClose    = 0x2000
	processSetQuota   = 0x0100
	processTerminate  = 0x0001
)

type basicLimits struct {
	perProcessUserTime int64
	perJobUserTime     int64
	flags              uint32
	minimumWorkingSet  uintptr
	maximumWorkingSet  uintptr
	activeProcesses    uint32
	affinity           uintptr
	priorityClass      uint32
	schedulingClass    uint32
}

type ioCounters struct {
	reads, writes, others                          uint64
	readBytes, writtenBytes, otherTransferredBytes uint64
}

type extendedLimits struct {
	basic             basicLimits
	io                ioCounters
	processMemory     uintptr
	jobMemory         uintptr
	peakProcessMemory uintptr
	peakJobMemory     uintptr
}

type kernel struct {
	createJob, setJob, assignProcess uintptr
}

var loadKernel = sync.OnceValues(func() (*kernel, error) {
	k := &kernel{}

	err := winapi.Load("kernel32.dll",
		winapi.Proc{Name: "CreateJobObjectW", Addr: &k.createJob},
		winapi.Proc{Name: "SetInformationJobObject", Addr: &k.setJob},
		winapi.Proc{Name: "AssignProcessToJobObject", Addr: &k.assignProcess},
	)
	if err != nil {
		return nil, err
	}

	return k, nil
})

// A console program started by a program without a console opens a window of its own.
func quiet(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}

// Windows ends every process in the job, the program's own children too, once the job's
// last handle closes, and that happens even when Sublayer itself is killed.
func contain(cmd *exec.Cmd) (kill func() error, err error) {
	job, err := newJob()
	if err != nil {
		return nil, errors.Join(err, cmd.Process.Kill())
	}

	if err := assign(job, cmd.Process.Pid); err != nil {
		return nil, errors.Join(err, cmd.Process.Kill(), syscall.CloseHandle(job))
	}

	return func() error { return syscall.CloseHandle(job) }, nil
}

func newJob() (syscall.Handle, error) {
	k, err := loadKernel()
	if err != nil {
		return 0, err
	}

	job, _, callErr := syscall.SyscallN(k.createJob, 0, 0)
	if job == 0 {
		return 0, fmt.Errorf("%w: CreateJobObjectW: %w", ErrJob, callErr)
	}

	limits := extendedLimits{basic: basicLimits{flags: killOnJobClose}}
	ok, _, callErr := syscall.SyscallN(k.setJob, job, extendedLimitInfo, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if ok == 0 {
		return 0, errors.Join(fmt.Errorf("%w: SetInformationJobObject: %w", ErrJob, callErr), syscall.CloseHandle(syscall.Handle(job)))
	}

	return syscall.Handle(job), nil
}

func assign(job syscall.Handle, pid int) error {
	k, err := loadKernel()
	if err != nil {
		return err
	}

	process, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("%w: OpenProcess: %w", ErrJob, err)
	}
	defer syscall.CloseHandle(process)

	ok, _, callErr := syscall.SyscallN(k.assignProcess, uintptr(job), uintptr(process))
	if ok == 0 {
		return fmt.Errorf("%w: AssignProcessToJobObject: %w", ErrJob, callErr)
	}

	return nil
}
