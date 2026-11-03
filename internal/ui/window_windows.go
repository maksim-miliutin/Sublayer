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
	swHide           = 0
	swShowNoActivate = 4
	ulwAlpha         = 0x2
	acSrcOver        = 0x00
	acSrcAlpha       = 0x01
	wmDestroy        = 0x0002
	wmMove           = 0x0003
	wmNcHitTest      = 0x0084
	htCaption        = 2
	gwlExStyle       = ^uintptr(19)
	wmClose          = 0x0010
	wmRedraw         = 0x8000
	spiGetWorkArea   = 0x0030

	className = "SublayerOverlay"
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
	setWindowLongPtr     uintptr
	monitorFromPoint     uintptr
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
		winapi.Proc{Name: "SetWindowLongPtrW", Addr: &u.setWindowLongPtr},
		winapi.Proc{Name: "MonitorFromPoint", Addr: &u.monitorFromPoint},
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

	openMu.Lock()
	w := opened[hwnd]
	openMu.Unlock()

	switch {
	case message == wmRedraw && w != nil:
		w.redraw()

		return 0
	case message == wmNcHitTest && w != nil && w.moving:
		return htCaption
	case message == wmMove && w != nil:
		w.moved(lParam)
	case message == wmDestroy:
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
	moving   bool
	look     Look
	mu       sync.Mutex
	corner   *Point
	placed   rect
	frame    Frame
	err      error
	closing  sync.Once
	done     chan struct{}
}

func Open(o Options) (Window, error) {
	u, err := loadUser()
	if err != nil {
		return nil, err
	}

	w := &window{user: u, look: o.Look, corner: o.Corner, done: make(chan struct{})}
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
	if err := winapi.AwareOfDPI(); err != nil {
		return err
	}
	dpi, _, _ := syscall.SyscallN(w.user.getDpiForSystem)

	r, err := newRenderer(int(dpi), w.look)
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

	style := exStyle(false)
	hwnd, _, callErr := syscall.SyscallN(w.user.createWindow, style, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		wsPopup, 0, 0, 1, 1, 0, 0, w.user.instance, 0)
	if hwnd == 0 {
		return failure("CreateWindowExW", callErr)
	}
	w.hwnd = hwnd

	openMu.Lock()
	opened[hwnd] = w
	openMu.Unlock()

	return w.refresh()
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

func (w *window) Corner() Point {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.corner != nil {
		return *w.corner
	}

	return Point{X: w.placed.x, Y: w.placed.bottom()}
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
	corner := w.corner
	w.mu.Unlock()

	var area winRect
	ok, _, callErr := syscall.SyscallN(w.user.systemParametersInfo, spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)
	if ok == 0 {
		return failure("SystemParametersInfoW", callErr)
	}

	work := rect{x: int(area.left), y: int(area.top), w: int(area.right - area.left), h: int(area.bottom - area.top)}
	l := layoutFor(work, w.renderer.metrics, w.look)

	c, err := w.renderer.draw(f, l)
	if err != nil {
		return err
	}
	defer c.free()

	at := l.at(c.scene.height, w.onScreen(corner))

	// Windows tells of every move, this one too, so the corner must already know the height.
	w.mu.Lock()
	w.placed = at
	w.mu.Unlock()
	dst := point{x: int32(at.x), y: int32(at.y)}
	extent := size{cx: int32(at.w), cy: int32(at.h)}
	var src point
	blend := blendFunction{op: acSrcOver, constantAlpha: 255, format: acSrcAlpha}

	ok, _, callErr = syscall.SyscallN(w.user.updateLayeredWindow, w.hwnd, 0, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&extent)),
		c.dc, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&blend)), ulwAlpha)
	if ok == 0 {
		return failure("UpdateLayeredWindow", callErr)
	}

	w.steer(f.Moving)

	shown := uintptr(swShowNoActivate)
	if f.Hidden {
		shown = swHide
	}
	syscall.SyscallN(w.user.showWindow, w.hwnd, shown)

	return nil
}

// Clicks pass through the subtitles except while they are being moved; then a drag on a
// plate moves them, because the window tells Windows that the plate is its title bar.
func exStyle(moving bool) uintptr {
	style := uintptr(wsExLayered | wsExTopmost | wsExToolWindow | wsExNoActivate)
	if !moving {
		style |= wsExTransparent
	}

	return style
}

func (w *window) steer(moving bool) {
	if moving == w.moving {
		return
	}

	w.moving = moving
	syscall.SyscallN(w.user.setWindowLongPtr, w.hwnd, gwlExStyle, exStyle(moving))
}

func (w *window) moved(lParam uintptr) {
	if !w.moving {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	x, y := int(int16(lParam&0xffff)), int(int16(lParam>>16&0xffff))
	w.corner = &Point{X: x, Y: y + w.placed.h}
}

func (w *window) onScreen(corner *Point) *Point {
	if corner == nil {
		return nil
	}

	// MonitorFromPoint takes the POINT itself, two 32-bit halves in one 64-bit word.
	packed := uint64(uint32(int32(corner.X))) | uint64(uint32(int32(corner.Y-1)))<<32
	monitor, _, _ := syscall.SyscallN(w.user.monitorFromPoint, uintptr(packed), 0)
	if monitor == 0 {
		return nil
	}

	return corner
}
