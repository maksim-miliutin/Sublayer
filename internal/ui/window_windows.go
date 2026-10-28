package ui

import (
	"runtime"
	"slices"
	"sync"
	"syscall"
	"unsafe"

	"github.com/maksim-miliutin/Sublayer/internal/winapi"
)

const (
	wsPopup          = 0x80000000
	wsExLayered      = 0x00080000
	wsExTransparent  = 0x00000020
	wsExTopmost      = 0x00000008
	wsExToolWindow   = 0x00000080
	wsExNoActivate   = 0x08000000
	swShowNoActivate = 4
	ulwAlpha         = 0x2
	acSrcOver        = 0x00
	acSrcAlpha       = 0x01
	wmDestroy        = 0x0002
	wmClose          = 0x0010
	wmRedraw         = 0x8000
	spiGetWorkArea   = 0x0030

	perMonitorAwareV2 = ^uintptr(3)
	className         = "SublayerOverlay"
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSmall  uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type point struct {
	x, y int32
}

type size struct {
	cx, cy int32
}

type blendFunction struct {
	op, flags, constantAlpha, format byte
}

type user struct {
	instance             uintptr
	registerClass        uintptr
	createWindow         uintptr
	defWindowProc        uintptr
	getMessage           uintptr
	translateMessage     uintptr
	dispatchMessage      uintptr
	postMessage          uintptr
	postQuitMessage      uintptr
	destroyWindow        uintptr
	showWindow           uintptr
	updateLayeredWindow  uintptr
	systemParametersInfo uintptr
	getDpiForSystem      uintptr
	setDpiAwareness      uintptr
}

var loadUser = sync.OnceValues(func() (*user, error) {
	u := &user{}

	err := winapi.Load("user32.dll",
		winapi.Proc{Name: "RegisterClassExW", Addr: &u.registerClass},
		winapi.Proc{Name: "CreateWindowExW", Addr: &u.createWindow},
		winapi.Proc{Name: "DefWindowProcW", Addr: &u.defWindowProc},
		winapi.Proc{Name: "GetMessageW", Addr: &u.getMessage},
		winapi.Proc{Name: "TranslateMessage", Addr: &u.translateMessage},
		winapi.Proc{Name: "DispatchMessageW", Addr: &u.dispatchMessage},
		winapi.Proc{Name: "PostMessageW", Addr: &u.postMessage},
		winapi.Proc{Name: "PostQuitMessage", Addr: &u.postQuitMessage},
		winapi.Proc{Name: "DestroyWindow", Addr: &u.destroyWindow},
		winapi.Proc{Name: "ShowWindow", Addr: &u.showWindow},
		winapi.Proc{Name: "UpdateLayeredWindow", Addr: &u.updateLayeredWindow},
		winapi.Proc{Name: "SystemParametersInfoW", Addr: &u.systemParametersInfo},
		winapi.Proc{Name: "GetDpiForSystem", Addr: &u.getDpiForSystem},
		winapi.Proc{Name: "SetProcessDpiAwarenessContext", Addr: &u.setDpiAwareness},
	)
	if err != nil {
		return nil, err
	}

	var moduleHandle uintptr
	if err := winapi.Load("kernel32.dll", winapi.Proc{Name: "GetModuleHandleW", Addr: &moduleHandle}); err != nil {
		return nil, err
	}

	u.instance, _, _ = syscall.SyscallN(moduleHandle, 0)

	return u, nil
})

var registerClass = sync.OnceValues(func() (*uint16, error) {
	u, err := loadUser()
	if err != nil {
		return nil, err
	}

	name, err := syscall.UTF16PtrFromString(className)
	if err != nil {
		return nil, err
	}

	class := wndClassEx{wndProc: syscall.NewCallback(dispatch), instance: u.instance, className: name}
	class.size = uint32(unsafe.Sizeof(class))

	atom, _, callErr := syscall.SyscallN(u.registerClass, uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return nil, failure("RegisterClassExW", callErr)
	}

	return name, nil
})

var (
	openMu sync.Mutex
	opened = map[uintptr]*window{}
)

// Windows calls a window procedure with exactly these four words, and NewCallback
// passes them on as they are, so they stay apart rather than gathered in a struct.
func dispatch(hwnd, message, wParam, lParam uintptr) uintptr {
	u, _ := loadUser()

	switch message {
	case wmRedraw:
		openMu.Lock()
		w := opened[hwnd]
		openMu.Unlock()

		if w != nil {
			w.redraw()
		}

		return 0
	case wmDestroy:
		openMu.Lock()
		delete(opened, hwnd)
		openMu.Unlock()

		syscall.SyscallN(u.postQuitMessage, 0)

		return 0
	}

	result, _, _ := syscall.SyscallN(u.defWindowProc, hwnd, message, wParam, lParam)

	return result
}

type window struct {
	user     *user
	renderer *renderer
	hwnd     uintptr
	mu       sync.Mutex
	frame    Frame
	err      error
	closing  sync.Once
	done     chan struct{}
}

func Open() (Window, error) {
	u, err := loadUser()
	if err != nil {
		return nil, err
	}

	w := &window{user: u, done: make(chan struct{})}
	ready := make(chan error)
	go w.run(ready)

	if err := <-ready; err != nil {
		return nil, err
	}

	return w, nil
}

// Windows queues a window's messages on the thread that made it. That thread is never
// unlocked, so it dies with the goroutine instead of carrying a stray WM_QUIT onward.
func (w *window) run(ready chan<- error) {
	runtime.LockOSThread()

	if err := w.create(); err != nil {
		if w.hwnd != 0 {
			syscall.SyscallN(w.user.destroyWindow, w.hwnd)
		}

		if w.renderer != nil {
			w.renderer.close()
		}

		ready <- err

		return
	}

	ready <- nil

	w.loop()
	w.renderer.close()
	close(w.done)
}

func (w *window) create() error {
	syscall.SyscallN(w.user.setDpiAwareness, perMonitorAwareV2)
	dpi, _, _ := syscall.SyscallN(w.user.getDpiForSystem)

	r, err := newRenderer(int(dpi))
	if err != nil {
		return err
	}
	w.renderer = r

	class, err := registerClass()
	if err != nil {
		return err
	}

	title, err := syscall.UTF16PtrFromString("Sublayer")
	if err != nil {
		return err
	}

	style := uintptr(wsExLayered | wsExTransparent | wsExTopmost | wsExToolWindow | wsExNoActivate)
	hwnd, _, callErr := syscall.SyscallN(w.user.createWindow, style, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		wsPopup, 0, 0, 1, 1, 0, 0, w.user.instance, 0)
	if hwnd == 0 {
		return failure("CreateWindowExW", callErr)
	}
	w.hwnd = hwnd

	openMu.Lock()
	opened[hwnd] = w
	openMu.Unlock()

	if err := w.refresh(); err != nil {
		return err
	}

	syscall.SyscallN(w.user.showWindow, hwnd, swShowNoActivate)

	return nil
}

func (w *window) loop() {
	var m msg
	for {
		got, _, _ := syscall.SyscallN(w.user.getMessage, uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(got) <= 0 {
			return
		}

		syscall.SyscallN(w.user.translateMessage, uintptr(unsafe.Pointer(&m)))
		syscall.SyscallN(w.user.dispatchMessage, uintptr(unsafe.Pointer(&m)))
	}
}

func (w *window) Show(f Frame) error {
	w.mu.Lock()
	w.frame = Frame{Pairs: slices.Clone(f.Pairs), State: f.State}
	err := w.err
	w.mu.Unlock()

	if err != nil {
		return err
	}

	posted, _, callErr := syscall.SyscallN(w.user.postMessage, w.hwnd, wmRedraw, 0, 0)
	if posted == 0 {
		return failure("PostMessageW", callErr)
	}

	return nil
}

func (w *window) Close() error {
	w.closing.Do(func() { syscall.SyscallN(w.user.postMessage, w.hwnd, wmClose, 0, 0) })
	<-w.done

	w.mu.Lock()
	defer w.mu.Unlock()

	return w.err
}

func (w *window) redraw() {
	err := w.refresh()
	if err == nil {
		return
	}

	w.mu.Lock()
	if w.err == nil {
		w.err = err
	}
	w.mu.Unlock()
}

func (w *window) refresh() error {
	w.mu.Lock()
	f := w.frame
	w.mu.Unlock()

	var area winRect
	ok, _, callErr := syscall.SyscallN(w.user.systemParametersInfo, spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)
	if ok == 0 {
		return failure("SystemParametersInfoW", callErr)
	}

	work := rect{x: int(area.left), y: int(area.top), w: int(area.right - area.left), h: int(area.bottom - area.top)}
	l := layoutFor(work, w.renderer.metrics)

	c, err := w.renderer.draw(f, l)
	if err != nil {
		return err
	}
	defer c.free()

	at := l.place(c.scene.height)
	dst := point{x: int32(at.x), y: int32(at.y)}
	extent := size{cx: int32(at.w), cy: int32(at.h)}
	var src point
	blend := blendFunction{op: acSrcOver, constantAlpha: 255, format: acSrcAlpha}

	ok, _, callErr = syscall.SyscallN(w.user.updateLayeredWindow, w.hwnd, 0, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&extent)),
		c.dc, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&blend)), ulwAlpha)
	if ok == 0 {
		return failure("UpdateLayeredWindow", callErr)
	}

	return nil
}
