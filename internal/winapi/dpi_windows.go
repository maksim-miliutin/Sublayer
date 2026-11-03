package winapi

import (
	"sync"
	"syscall"
)

const perMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2, which Windows defines as the handle -4

var setAwareness = sync.OnceValues(func() (uintptr, error) {
	var set uintptr
	if err := Load("user32.dll", Proc{Name: "SetProcessDpiAwarenessContext", Addr: &set}); err != nil {
		return 0, err
	}

	return set, nil
})

func AwareOfDPI() error {
	set, err := setAwareness()
	if err != nil {
		return err
	}

	syscall.SyscallN(set, perMonitorAwareV2)

	return nil
}
