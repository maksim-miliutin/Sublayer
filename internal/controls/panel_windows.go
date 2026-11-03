package controls

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/maksim-miliutin/Sublayer/internal/ui"
	"github.com/maksim-miliutin/Sublayer/internal/winapi"
)

const (
	wmNull         = 0x0000
	wmDestroy      = 0x0002
	wmClose        = 0x0010
	wmLButtonUp    = 0x0202
	wmRButtonUp    = 0x0205
	wmTray         = 0x8001
	wmRefresh      = 0x8002
	nimAdd         = 0x0
	nimModify      = 0x1
	nimDelete      = 0x2
	nifMessage     = 0x1
	nifIcon        = 0x2
	nifTip         = 0x4
	nifInfo        = 0x10
	niifWarning    = 0x2
	mfGrayed       = 0x1
	mfChecked      = 0x8
	mfSeparator    = 0x800
	tpmRightButton = 0x2
	tpmNoNotify    = 0x80
	tpmReturnCmd   = 0x100
	smCxSmIcon     = 49
	biBitfields    = 3
	iconID         = 1
	className      = "SublayerControls"
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

type point struct {
	x, y int32
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type notifyIconData struct {
	size            uint32
	window          uintptr
	id              uint32
	flags           uint32
	callbackMessage uint32
	icon            uintptr
	tip             [128]uint16
	state           uint32
	stateMask       uint32
	info            [256]uint16
	version         uint32
	infoTitle       [64]uint16
	infoFlags       uint32
	guidItem        [16]byte
	balloonIcon     uintptr
}

type bitmapV5Header struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	colorsUsed    uint32
	colorsNeeded  uint32
	redMask       uint32
	greenMask     uint32
	blueMask      uint32
	alphaMask     uint32
	colorSpace    uint32
	endpoints     [36]byte
	gammaRed      uint32
	gammaGreen    uint32
	gammaBlue     uint32
	intent        uint32
	profileData   uint32
	profileSize   uint32
	reserved      uint32
}

type iconInfo struct {
	isIcon   int32
	hotspotX uint32
	hotspotY uint32
	mask     uintptr
	colour   uintptr
}

type api struct {
	instance                                                  uintptr
	registerClass, createWindow, defWindowProc, destroyWindow uintptr
	getMessage, translateMessage, dispatchMessage             uintptr
	postMessage, postQuitMessage, registerWindowMessage       uintptr
	createPopupMenu, appendMenu, trackPopupMenu, destroyMenu  uintptr
	setForegroundWindow, getCursorPos, getSystemMetrics       uintptr
	createIconIndirect, destroyIcon, getDC, releaseDC         uintptr
	notifyIcon, createDIBSection, createBitmap, deleteObject  uintptr
}

var loadAPI = sync.OnceValues(func() (*api, error) {
	a := &api{}

	var moduleHandle uintptr
	err := winapi.Load("user32.dll",
		winapi.Proc{Name: "RegisterClassExW", Addr: &a.registerClass},
		winapi.Proc{Name: "CreateWindowExW", Addr: &a.createWindow},
		winapi.Proc{Name: "DefWindowProcW", Addr: &a.defWindowProc},
		winapi.Proc{Name: "DestroyWindow", Addr: &a.destroyWindow},
		winapi.Proc{Name: "GetMessageW", Addr: &a.getMessage},
		winapi.Proc{Name: "TranslateMessage", Addr: &a.translateMessage},
		winapi.Proc{Name: "DispatchMessageW", Addr: &a.dispatchMessage},
		winapi.Proc{Name: "PostMessageW", Addr: &a.postMessage},
		winapi.Proc{Name: "PostQuitMessage", Addr: &a.postQuitMessage},
		winapi.Proc{Name: "RegisterWindowMessageW", Addr: &a.registerWindowMessage},
		winapi.Proc{Name: "CreatePopupMenu", Addr: &a.createPopupMenu},
		winapi.Proc{Name: "AppendMenuW", Addr: &a.appendMenu},
		winapi.Proc{Name: "TrackPopupMenu", Addr: &a.trackPopupMenu},
		winapi.Proc{Name: "DestroyMenu", Addr: &a.destroyMenu},
		winapi.Proc{Name: "SetForegroundWindow", Addr: &a.setForegroundWindow},
		winapi.Proc{Name: "GetCursorPos", Addr: &a.getCursorPos},
		winapi.Proc{Name: "GetSystemMetrics", Addr: &a.getSystemMetrics},
		winapi.Proc{Name: "CreateIconIndirect", Addr: &a.createIconIndirect},
		winapi.Proc{Name: "DestroyIcon", Addr: &a.destroyIcon},
		winapi.Proc{Name: "GetDC", Addr: &a.getDC},
		winapi.Proc{Name: "ReleaseDC", Addr: &a.releaseDC},
	)
	if err == nil {
		err = winapi.Load("shell32.dll", winapi.Proc{Name: "Shell_NotifyIconW", Addr: &a.notifyIcon})
	}
	if err == nil {
		err = winapi.Load("gdi32.dll",
			winapi.Proc{Name: "CreateDIBSection", Addr: &a.createDIBSection},
			winapi.Proc{Name: "CreateBitmap", Addr: &a.createBitmap},
			winapi.Proc{Name: "DeleteObject", Addr: &a.deleteObject},
		)
	}
	if err == nil {
		err = winapi.Load("kernel32.dll", winapi.Proc{Name: "GetModuleHandleW", Addr: &moduleHandle})
	}
	if err != nil {
		return nil, err
	}

	a.instance, _, _ = syscall.SyscallN(moduleHandle, 0)

	return a, nil
})

var registerClass = sync.OnceValues(func() (*uint16, error) {
	a, err := loadAPI()
	if err != nil {
		return nil, err
	}

	name, err := syscall.UTF16PtrFromString(className)
	if err != nil {
		return nil, err
	}

	class := wndClassEx{wndProc: syscall.NewCallback(dispatch), instance: a.instance, className: name}
	class.size = uint32(unsafe.Sizeof(class))

	atom, _, callErr := syscall.SyscallN(a.registerClass, uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return nil, failure("RegisterClassExW", callErr)
	}

	return name, nil
})

var (
	openMu sync.Mutex
	opened = map[uintptr]*panel{}
)

type panel struct {
	api       *api
	languages []string
	hwnd      uintptr
	taskbar   uintptr
	icons     map[ui.State]uintptr
	choices   chan Choice
	done      chan struct{}
	closing   sync.Once
	mu        sync.Mutex
	status    Status
	alert     string
	err       error
}

func Open(languages []string) (Panel, error) {
	a, err := loadAPI()
	if err != nil {
		return nil, err
	}

	p := &panel{api: a, languages: languages, icons: map[ui.State]uintptr{}, choices: make(chan Choice, 8), done: make(chan struct{})}
	ready := make(chan error)
	go p.run(ready)

	if err := <-ready; err != nil {
		return nil, err
	}

	return p, nil
}

func (p *panel) Show(s Status) error {
	p.mu.Lock()
	p.status = s
	err := p.err
	p.mu.Unlock()

	if err != nil {
		return err
	}

	return p.post(wmRefresh)
}

func (p *panel) Alert(text string) error {
	p.mu.Lock()
	p.alert = text
	err := p.err
	p.mu.Unlock()

	if err != nil {
		return err
	}

	return p.post(wmRefresh)
}

func (p *panel) Choices() <-chan Choice {
	return p.choices
}

func (p *panel) Close() error {
	p.closing.Do(func() { syscall.SyscallN(p.api.postMessage, p.hwnd, wmClose, 0, 0) })
	<-p.done

	p.mu.Lock()
	defer p.mu.Unlock()

	return p.err
}

// Windows queues a window's messages on the thread that made it. That thread is never
// unlocked, so it dies with the goroutine instead of carrying a stray WM_QUIT onward.
func (p *panel) run(ready chan<- error) {
	runtime.LockOSThread()

	if err := p.create(); err != nil {
		if p.hwnd != 0 {
			syscall.SyscallN(p.api.destroyWindow, p.hwnd)
		}

		ready <- err

		return
	}

	ready <- nil

	p.loop()
	close(p.choices)
	close(p.done)
}

func (p *panel) create() error {
	if err := winapi.AwareOfDPI(); err != nil {
		return err
	}

	class, err := registerClass()
	if err != nil {
		return err
	}

	title, err := syscall.UTF16PtrFromString("Sublayer")
	if err != nil {
		return err
	}

	hwnd, _, callErr := syscall.SyscallN(p.api.createWindow, 0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 0, 0, 0, 0, p.api.instance, 0)
	if hwnd == 0 {
		return failure("CreateWindowExW", callErr)
	}
	p.hwnd = hwnd

	openMu.Lock()
	opened[hwnd] = p
	openMu.Unlock()

	taskbar, err := syscall.UTF16PtrFromString("TaskbarCreated")
	if err != nil {
		return err
	}
	p.taskbar, _, _ = syscall.SyscallN(p.api.registerWindowMessage, uintptr(unsafe.Pointer(taskbar)))

	return p.notify(nimAdd)
}

func (p *panel) loop() {
	var m msg
	for {
		got, _, _ := syscall.SyscallN(p.api.getMessage, uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(got) <= 0 {
			return
		}

		syscall.SyscallN(p.api.translateMessage, uintptr(unsafe.Pointer(&m)))
		syscall.SyscallN(p.api.dispatchMessage, uintptr(unsafe.Pointer(&m)))
	}
}

// Windows calls a window procedure with exactly these four words, and NewCallback
// passes them on as they are, so they stay apart rather than gathered in a struct.
func dispatch(hwnd, message, wParam, lParam uintptr) uintptr {
	openMu.Lock()
	p := opened[hwnd]
	openMu.Unlock()

	a, _ := loadAPI()
	if p == nil {
		result, _, _ := syscall.SyscallN(a.defWindowProc, hwnd, message, wParam, lParam)

		return result
	}

	click := lParam & 0xffff
	switch {
	case message == wmTray && (click == wmRButtonUp || click == wmLButtonUp):
		p.popup()
	case message == wmRefresh:
		p.record(p.notify(nimModify))
	case message == p.taskbar && p.taskbar != 0:
		p.record(p.notify(nimAdd))
	case message == wmDestroy:
		p.destroy()
	default:
		result, _, _ := syscall.SyscallN(a.defWindowProc, hwnd, message, wParam, lParam)

		return result
	}

	return 0
}

func (p *panel) destroy() {
	data := notifyIconData{window: p.hwnd, id: iconID}
	data.size = uint32(unsafe.Sizeof(data))
	syscall.SyscallN(p.api.notifyIcon, nimDelete, uintptr(unsafe.Pointer(&data)))

	for _, icon := range p.icons {
		syscall.SyscallN(p.api.destroyIcon, icon)
	}

	openMu.Lock()
	delete(opened, p.hwnd)
	openMu.Unlock()

	syscall.SyscallN(p.api.postQuitMessage, 0)
}

func (p *panel) notify(operation uintptr) error {
	p.mu.Lock()
	status, alert := p.status, p.alert
	p.alert = ""
	p.mu.Unlock()

	icon, err := p.iconFor(status.State)
	if err != nil {
		return err
	}

	data := notifyIconData{window: p.hwnd, id: iconID, flags: nifMessage | nifIcon | nifTip, callbackMessage: wmTray, icon: icon}
	data.size = uint32(unsafe.Sizeof(data))
	fill(data.tip[:], tip(status))

	if alert != "" {
		data.flags |= nifInfo
		data.infoFlags = niifWarning
		fill(data.info[:], alert)
		fill(data.infoTitle[:], "Sublayer")
	}

	ok, _, callErr := syscall.SyscallN(p.api.notifyIcon, operation, uintptr(unsafe.Pointer(&data)))
	if ok == 0 {
		return failure("Shell_NotifyIconW", callErr)
	}

	return nil
}

func (p *panel) iconFor(state ui.State) (uintptr, error) {
	if icon, made := p.icons[state]; made {
		return icon, nil
	}

	side, _, _ := syscall.SyscallN(p.api.getSystemMetrics, smCxSmIcon)

	icon, err := p.makeIcon(int(side), icon(int(side), ui.DotColor(state)))
	if err != nil {
		return 0, err
	}
	p.icons[state] = icon

	return icon, nil
}

func (p *panel) makeIcon(side int, pixels []byte) (uintptr, error) {
	header := bitmapV5Header{
		width:       int32(side),
		height:      -int32(side),
		planes:      1,
		bitCount:    32,
		compression: biBitfields,
		redMask:     0x00ff0000,
		greenMask:   0x0000ff00,
		blueMask:    0x000000ff,
		alphaMask:   0xff000000,
	}
	header.size = uint32(unsafe.Sizeof(header))

	screen, _, _ := syscall.SyscallN(p.api.getDC, 0)
	var bits unsafe.Pointer
	colour, _, callErr := syscall.SyscallN(p.api.createDIBSection, screen, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	syscall.SyscallN(p.api.releaseDC, 0, screen)
	if colour == 0 {
		return 0, failure("CreateDIBSection", callErr)
	}
	defer syscall.SyscallN(p.api.deleteObject, colour)

	copy(unsafe.Slice((*byte)(bits), len(pixels)), pixels)

	// Left unwritten, a bitmap's bits are whatever was there; a clear mask lets the alpha decide.
	clear := make([]byte, (side+15)/16*2*side)
	mask, _, callErr := syscall.SyscallN(p.api.createBitmap, uintptr(side), uintptr(side), 1, 1, uintptr(unsafe.Pointer(&clear[0])))
	if mask == 0 {
		return 0, failure("CreateBitmap", callErr)
	}
	defer syscall.SyscallN(p.api.deleteObject, mask)

	info := iconInfo{isIcon: 1, mask: mask, colour: colour}
	icon, _, callErr := syscall.SyscallN(p.api.createIconIndirect, uintptr(unsafe.Pointer(&info)))
	if icon == 0 {
		return 0, failure("CreateIconIndirect", callErr)
	}

	return icon, nil
}

func (p *panel) popup() {
	p.mu.Lock()
	items := menu(p.status, p.languages)
	p.mu.Unlock()

	picked, err := p.track(items)
	if err != nil {
		p.record(err)

		return
	}

	for _, it := range items {
		if it.id != 0 && it.id == picked {
			p.choose(it.choice)
		}
	}
}

func (p *panel) track(items []item) (int, error) {
	popup, _, callErr := syscall.SyscallN(p.api.createPopupMenu)
	if popup == 0 {
		return 0, failure("CreatePopupMenu", callErr)
	}
	defer syscall.SyscallN(p.api.destroyMenu, popup)

	for _, it := range items {
		if err := p.add(popup, it); err != nil {
			return 0, err
		}
	}

	var at point
	syscall.SyscallN(p.api.getCursorPos, uintptr(unsafe.Pointer(&at)))

	// A menu whose window is not in the foreground stays open after a click elsewhere,
	// and Windows closes it properly only once the window gets a message after it.
	syscall.SyscallN(p.api.setForegroundWindow, p.hwnd)
	picked, _, _ := syscall.SyscallN(p.api.trackPopupMenu, popup, tpmReturnCmd|tpmRightButton|tpmNoNotify, uintptr(at.x), uintptr(at.y), 0, p.hwnd, 0)
	syscall.SyscallN(p.api.postMessage, p.hwnd, wmNull, 0, 0)

	return int(picked), nil
}

func (p *panel) add(popup uintptr, it item) error {
	flags, label, err := entry(it)
	if err != nil {
		return err
	}

	ok, _, callErr := syscall.SyscallN(p.api.appendMenu, popup, flags, uintptr(it.id), uintptr(unsafe.Pointer(label)))
	if ok == 0 {
		return failure("AppendMenuW", callErr)
	}

	return nil
}

func entry(it item) (flags uintptr, label *uint16, err error) {
	if it.line {
		return mfSeparator, nil, nil
	}

	label, err = syscall.UTF16PtrFromString(it.label)
	if err != nil {
		return 0, nil, err
	}

	if it.checked {
		flags |= mfChecked
	}
	if it.title {
		flags |= mfGrayed
	}

	return flags, label, nil
}

func (p *panel) choose(c Choice) {
	select {
	case p.choices <- c:
	default:
	}
}

func (p *panel) post(message uintptr) error {
	posted, _, callErr := syscall.SyscallN(p.api.postMessage, p.hwnd, message, 0, 0)
	if posted == 0 {
		return failure("PostMessageW", callErr)
	}

	return nil
}

func (p *panel) record(err error) {
	if err == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.err == nil {
		p.err = err
	}
}

func failure(name string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrTray, name, err)
}
