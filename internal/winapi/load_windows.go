package winapi

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

var ErrMissing = errors.New("winapi: a system function is missing")

type Proc struct {
	Name string
	Addr *uintptr
}

const searchSystem32 = 0x800

var loadLibraryEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")

func Load(dll string, procs ...Proc) error {
	name, err := syscall.UTF16PtrFromString(dll)
	if err != nil {
		return err
	}

	// syscall would look for a DLL it does not know in the program's folder first;
	// these belong in System32 and are taken from nowhere else.
	module, _, callErr := loadLibraryEx.Call(uintptr(unsafe.Pointer(name)), 0, searchSystem32)
	if module == 0 {
		return fmt.Errorf("%w: %s: %w", ErrMissing, dll, callErr)
	}

	for _, p := range procs {
		addr, err := syscall.GetProcAddress(syscall.Handle(module), p.Name)
		if err != nil {
			return fmt.Errorf("%w: %s in %s: %w", ErrMissing, p.Name, dll, err)
		}

		*p.Addr = addr
	}

	return nil
}
