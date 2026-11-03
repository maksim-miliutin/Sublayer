package main

import (
	"syscall"
	"unsafe"

	"github.com/maksim-miliutin/Sublayer/internal/winapi"
)

const mbIconError = 0x10

// Started by a double click, Sublayer gets a console of its own; letting it go leaves no
// black window beside the subtitles, while one shared with a shell stays to be written to.
func detach() bool {
	var processes, free uintptr
	err := winapi.Load("kernel32.dll",
		winapi.Proc{Name: "GetConsoleProcessList", Addr: &processes},
		winapi.Proc{Name: "FreeConsole", Addr: &free},
	)
	if err != nil {
		return false
	}

	var attached [2]uint32
	count, _, _ := syscall.SyscallN(processes, uintptr(unsafe.Pointer(&attached[0])), uintptr(len(attached)))
	if count != 1 {
		return false
	}

	syscall.SyscallN(free)

	return true
}

func alarm(failure error) {
	var box uintptr
	if winapi.Load("user32.dll", winapi.Proc{Name: "MessageBoxW", Addr: &box}) != nil {
		return
	}

	text, textErr := syscall.UTF16PtrFromString(failure.Error())
	title, titleErr := syscall.UTF16PtrFromString("Sublayer")
	if textErr != nil || titleErr != nil {
		return
	}

	syscall.SyscallN(box, 0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbIconError)
}
