package form

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/maksim-miliutin/Sublayer/internal/winapi"
)

var ErrWindow = errors.New("form: the settings window failed")

const (
	wsCaption               = 0x00C00000
	wsSysMenu               = 0x00080000
	wsMinimizeBox           = 0x00020000
	wsChild                 = 0x40000000
	wsVisible               = 0x10000000
	wsTabStop               = 0x00010000
	wsVScroll               = 0x00200000
	wsClipSiblings          = 0x04000000
	wsExClientEdge          = 0x00000200
	esAutoHScroll           = 0x0080
	esNumber                = 0x2000
	bsDefPushButton         = 0x0001
	bsGroupBox              = 0x0007
	bnClicked               = 0
	ssCenterImage           = 0x0200
	ssEndEllipsis           = 0x4000
	cbsDropDownList         = 0x0003
	cbAddString             = 0x0143
	cbGetCurSel             = 0x0147
	cbSetCurSel             = 0x014E
	wmDestroy               = 0x0002
	wmClose                 = 0x0010
	wmSetFont               = 0x0030
	wmCommand               = 0x0111
	wmDpiChanged            = 0x02E0
	idOK                    = 1
	idCancel                = 2
	cwUseDefault            = 0x80000000
	swShowNormal            = 1
	swRestore               = 9
	swpNoMove               = 0x0002
	swpNoZOrder             = 0x0004
	swpNoActivate           = 0x0010
	colorBtnFace            = 15
	idcArrow                = 32512
	mbIconWarning           = 0x30
	fwNormal                = 400
	defaultCharset          = 1
	ofnHideReadOnly         = 0x00000004
	ofnNoChangeDir          = 0x00000008
	ofnPathMustExist        = 0x00000800
	ofnFileMustExist        = 0x00001000
	ofnExplorer             = 0x00080000
	coinitApartmentThreaded = 0x2
	coinitDisableOLE1DDE    = 0x4

	windowStyle = wsCaption | wsSysMenu | wsMinimizeBox
	inputBase   = 100
	browseBase  = 1000
	dropRows    = 6
	fontPoints  = 9
	fileUnits   = 1024
	className   = "SublayerSettings"
)

var filters = map[Kind]string{
	Program: "Programs (*.exe)\x00*.exe\x00All files (*.*)\x00*.*\x00\x00",
	Model:   "Whisper models (*.bin)\x00*.bin\x00All files (*.*)\x00*.*\x00\x00",
}

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

type winRect struct {
	left, top, right, bottom int32
}

type openFileName struct {
	structSize    uint32
	owner         uintptr
	instance      uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reserved      uintptr
	reservedWord  uint32
	flagsEx       uint32
}

type api struct {
	instance                                                       uintptr
	registerClass, createWindow, defWindowProc, destroyWindow      uintptr
	getMessage, isDialogMessage, translateMessage, dispatchMessage uintptr
	postMessage, postQuitMessage, sendMessage                      uintptr
	showWindow, setForegroundWindow, setFocus                      uintptr
	setWindowPos, moveWindow, getDpiForWindow, adjustWindowRect    uintptr
	getWindowText, getWindowTextLength, setWindowText              uintptr
	messageBox, loadCursor, createFont, deleteObject               uintptr
	openFile, dialogError, initializeCOM, uninitializeCOM          uintptr
}

var loadAPI = sync.OnceValues(func() (*api, error) {
	a := &api{}
	var moduleHandle uintptr

	err := errors.Join(
		winapi.Load("user32.dll",
			winapi.Proc{Name: "RegisterClassExW", Addr: &a.registerClass},
			winapi.Proc{Name: "CreateWindowExW", Addr: &a.createWindow},
			winapi.Proc{Name: "DefWindowProcW", Addr: &a.defWindowProc},
			winapi.Proc{Name: "DestroyWindow", Addr: &a.destroyWindow},
			winapi.Proc{Name: "GetMessageW", Addr: &a.getMessage},
			winapi.Proc{Name: "IsDialogMessageW", Addr: &a.isDialogMessage},
			winapi.Proc{Name: "TranslateMessage", Addr: &a.translateMessage},
			winapi.Proc{Name: "DispatchMessageW", Addr: &a.dispatchMessage},
			winapi.Proc{Name: "PostMessageW", Addr: &a.postMessage},
			winapi.Proc{Name: "PostQuitMessage", Addr: &a.postQuitMessage},
			winapi.Proc{Name: "SendMessageW", Addr: &a.sendMessage},
			winapi.Proc{Name: "ShowWindow", Addr: &a.showWindow},
			winapi.Proc{Name: "SetForegroundWindow", Addr: &a.setForegroundWindow},
			winapi.Proc{Name: "SetFocus", Addr: &a.setFocus},
			winapi.Proc{Name: "SetWindowPos", Addr: &a.setWindowPos},
			winapi.Proc{Name: "MoveWindow", Addr: &a.moveWindow},
			winapi.Proc{Name: "GetDpiForWindow", Addr: &a.getDpiForWindow},
			winapi.Proc{Name: "AdjustWindowRectExForDpi", Addr: &a.adjustWindowRect},
			winapi.Proc{Name: "GetWindowTextW", Addr: &a.getWindowText},
			winapi.Proc{Name: "GetWindowTextLengthW", Addr: &a.getWindowTextLength},
			winapi.Proc{Name: "SetWindowTextW", Addr: &a.setWindowText},
			winapi.Proc{Name: "MessageBoxW", Addr: &a.messageBox},
			winapi.Proc{Name: "LoadCursorW", Addr: &a.loadCursor},
		),
		winapi.Load("gdi32.dll",
			winapi.Proc{Name: "CreateFontW", Addr: &a.createFont},
			winapi.Proc{Name: "DeleteObject", Addr: &a.deleteObject},
		),
		winapi.Load("comdlg32.dll",
			winapi.Proc{Name: "GetOpenFileNameW", Addr: &a.openFile},
			winapi.Proc{Name: "CommDlgExtendedError", Addr: &a.dialogError},
		),
		winapi.Load("ole32.dll",
			winapi.Proc{Name: "CoInitializeEx", Addr: &a.initializeCOM},
			winapi.Proc{Name: "CoUninitialize", Addr: &a.uninitializeCOM},
		),
		winapi.Load("kernel32.dll", winapi.Proc{Name: "GetModuleHandleW", Addr: &moduleHandle}),
	)
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

	cursor, _, _ := syscall.SyscallN(a.loadCursor, 0, idcArrow)
	class := wndClassEx{wndProc: syscall.NewCallback(dispatch), instance: a.instance, cursor: cursor, background: colorBtnFace + 1, className: name}
	class.size = uint32(unsafe.Sizeof(class))

	atom, _, callErr := syscall.SyscallN(a.registerClass, uintptr(unsafe.Pointer(&class)))
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
	a, _ := loadAPI()

	openMu.Lock()
	w := opened[hwnd]
	openMu.Unlock()

	switch {
	case message == wmCommand && w != nil && wParam>>16&0xffff == bnClicked:
		w.command(int(wParam & 0xffff))

		return 0
	case message == wmDpiChanged && w != nil:
		w.rescale(int(wParam & 0xffff))

		return 0
	case message == wmDestroy:
		openMu.Lock()
		delete(opened, hwnd)
		openMu.Unlock()

		syscall.SyscallN(a.postQuitMessage, 0)

		return 0
	}

	result, _, _ := syscall.SyscallN(a.defWindowProc, hwnd, message, wParam, lParam)

	return result
}

type Editor struct {
	mu   sync.Mutex
	open *window
}

func (e *Editor) Edit(c Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.open != nil && !e.open.gone() {
		e.open.raise()

		return nil
	}

	w, err := open(c)
	if err != nil {
		return err
	}
	e.open = w

	return nil
}

func (e *Editor) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.open != nil {
		e.open.close()
	}
}

type spec struct {
	class, text     string
	style, extended uintptr
	id              int
}

type part struct {
	hwnd     uintptr
	at       func(plan) box
	dropDown bool // a combo box, whose height counts the list it drops
}

type input struct {
	section, field int
	hwnd           uintptr
}

type window struct {
	api      *api
	config   Config
	sections []Section
	hwnd     uintptr
	font     uintptr
	parts    []part
	inputs   []input
	done     chan struct{}
}

func open(c Config) (*window, error) {
	a, err := loadAPI()
	if err != nil {
		return nil, err
	}

	shown, err := Sections(c.Settings)
	if err != nil {
		return nil, err
	}

	w := &window{api: a, config: c, sections: shown, done: make(chan struct{})}
	ready := make(chan error)
	go w.run(ready)

	if err := <-ready; err != nil {
		return nil, err
	}

	return w, nil
}

// The thread is never unlocked, so it dies with the goroutine instead of carrying a stray
// WM_QUIT onward, and the file dialog wants it in a single-threaded COM apartment.
func (w *window) run(ready chan<- error) {
	runtime.LockOSThread()
	defer close(w.done)

	joined, _, _ := syscall.SyscallN(w.api.initializeCOM, 0, coinitApartmentThreaded|coinitDisableOLE1DDE)
	if int32(joined) >= 0 {
		defer syscall.SyscallN(w.api.uninitializeCOM)
	}
	defer w.release()

	if err := w.create(); err != nil {
		if w.hwnd != 0 {
			syscall.SyscallN(w.api.destroyWindow, w.hwnd)
		}
		ready <- err

		return
	}
	ready <- nil

	w.loop()
}

func (w *window) create() error {
	if err := winapi.AwareOfDPI(); err != nil {
		return err
	}

	class, err := registerClass()
	if err != nil {
		return err
	}

	title, err := syscall.UTF16PtrFromString("Sublayer settings")
	if err != nil {
		return err
	}

	hwnd, _, callErr := syscall.SyscallN(w.api.createWindow, 0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		windowStyle, cwUseDefault, cwUseDefault, 0, 0, 0, 0, w.api.instance, 0)
	if hwnd == 0 {
		return failure("CreateWindowExW", callErr)
	}
	w.hwnd = hwnd

	openMu.Lock()
	opened[hwnd] = w
	openMu.Unlock()

	if err := w.build(); err != nil {
		return err
	}

	dpi, _, _ := syscall.SyscallN(w.api.getDpiForWindow, hwnd)
	if err := w.scale(int(dpi)); err != nil {
		return err
	}

	syscall.SyscallN(w.api.showWindow, hwnd, swShowNormal)
	syscall.SyscallN(w.api.setForegroundWindow, hwnd)
	syscall.SyscallN(w.api.setFocus, w.inputs[0].hwnd)

	return nil
}

func (w *window) build() error {
	for i := range w.sections {
		if err := w.buildSection(i); err != nil {
			return err
		}
	}

	bar := []struct {
		spec spec
		at   func(plan) box
	}{
		{spec{class: "STATIC", text: w.config.Note, style: ssCenterImage | ssEndEllipsis}, func(p plan) box { return p.note }},
		{spec{class: "BUTTON", text: "Save", style: wsTabStop | bsDefPushButton, id: idOK}, func(p plan) box { return p.save }},
		{spec{class: "BUTTON", text: "Cancel", style: wsTabStop, id: idCancel}, func(p plan) box { return p.cancel }},
	}
	for _, b := range bar {
		if _, err := w.place(b.spec, b.at); err != nil {
			return err
		}
	}

	// Made last, the frames lie under the fields in the z-order, and WS_CLIPSIBLINGS keeps
	// their painting off the fields.
	for i, section := range w.sections {
		if _, err := w.place(spec{class: "BUTTON", text: section.Title, style: bsGroupBox | wsClipSiblings}, func(p plan) box { return p.frames[i] }); err != nil {
			return err
		}
	}

	return nil
}

func (w *window) buildSection(i int) error {
	for j, f := range w.sections[i].Fields {
		if err := w.buildField(i, j, f); err != nil {
			return err
		}
	}

	return nil
}

func (w *window) buildField(i, j int, f Field) error {
	label := spec{class: "STATIC", text: f.Label, style: ssCenterImage | ssEndEllipsis}
	if _, err := w.place(label, func(p plan) box { return p.rows[i][j].label }); err != nil {
		return err
	}

	control, err := w.place(inputSpec(f, inputBase+len(w.inputs)), func(p plan) box { return p.rows[i][j].input })
	if err != nil {
		return err
	}
	w.inputs = append(w.inputs, input{section: i, field: j, hwnd: control})

	if err := w.offer(control, f); err != nil {
		return err
	}

	if f.Kind != Program && f.Kind != Model {
		return nil
	}

	browse := spec{class: "BUTTON", text: "Browse…", style: wsTabStop, id: browseBase + len(w.inputs) - 1}
	_, err = w.place(browse, func(p plan) box { return p.rows[i][j].browse })

	return err
}

func inputSpec(f Field, id int) spec {
	switch f.Kind {
	case Choice:
		return spec{class: "COMBOBOX", style: wsTabStop | wsVScroll | cbsDropDownList, id: id}
	case Number:
		return spec{class: "EDIT", text: f.Value, style: wsTabStop | esAutoHScroll | esNumber, extended: wsExClientEdge, id: id}
	}

	return spec{class: "EDIT", text: f.Value, style: wsTabStop | esAutoHScroll, extended: wsExClientEdge, id: id}
}

func (w *window) offer(combo uintptr, f Field) error {
	for i, o := range f.Options {
		name, err := syscall.UTF16PtrFromString(o.Name)
		if err != nil {
			return err
		}
		syscall.SyscallN(w.api.sendMessage, combo, cbAddString, 0, uintptr(unsafe.Pointer(name)))

		if o.Value == f.Value {
			syscall.SyscallN(w.api.sendMessage, combo, cbSetCurSel, uintptr(i), 0)
		}
	}

	return nil
}

func (w *window) place(s spec, at func(plan) box) (uintptr, error) {
	class, err := syscall.UTF16PtrFromString(s.class)
	if err != nil {
		return 0, err
	}

	text, err := syscall.UTF16PtrFromString(s.text)
	if err != nil {
		return 0, err
	}

	hwnd, _, callErr := syscall.SyscallN(w.api.createWindow, s.extended, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(text)),
		wsChild|wsVisible|s.style, 0, 0, 0, 0, w.hwnd, uintptr(s.id), w.api.instance, 0)
	if hwnd == 0 {
		return 0, failure("CreateWindowExW "+s.class, callErr)
	}
	w.parts = append(w.parts, part{hwnd: hwnd, at: at, dropDown: s.class == "COMBOBOX"})

	return hwnd, nil
}

func (w *window) scale(dpi int) error {
	face, err := syscall.UTF16PtrFromString("Segoe UI")
	if err != nil {
		return err
	}

	height := -(fontPoints*dpi + 36) / 72
	font, _, callErr := syscall.SyscallN(w.api.createFont, uintptr(height), 0, 0, 0, fwNormal, 0, 0, 0, defaultCharset, 0, 0, 0, 0, uintptr(unsafe.Pointer(face)))
	if font == 0 {
		return failure("CreateFontW", callErr)
	}

	p := arrange(w.sections, dpi)
	for _, part := range w.parts {
		b := part.at(p)
		if part.dropDown {
			b.h *= dropRows
		}

		syscall.SyscallN(w.api.moveWindow, part.hwnd, uintptr(b.x), uintptr(b.y), uintptr(b.w), uintptr(b.h), 1)
		syscall.SyscallN(w.api.sendMessage, part.hwnd, wmSetFont, font, 1)
	}
	w.release()
	w.font = font

	outer := winRect{right: int32(p.width), bottom: int32(p.height)}
	syscall.SyscallN(w.api.adjustWindowRect, uintptr(unsafe.Pointer(&outer)), windowStyle, 0, 0, uintptr(dpi))
	syscall.SyscallN(w.api.setWindowPos, w.hwnd, 0, 0, 0, uintptr(outer.right-outer.left), uintptr(outer.bottom-outer.top), swpNoMove|swpNoZOrder|swpNoActivate)

	return nil
}

func (w *window) rescale(dpi int) {
	if err := w.scale(dpi); err != nil {
		w.tell(err)
	}
}

func (w *window) loop() {
	var m msg
	for {
		got, _, _ := syscall.SyscallN(w.api.getMessage, uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(got) <= 0 {
			return
		}

		// IsDialogMessage gives the window the keys of a dialog: Tab from field to field,
		// Enter for Save and Esc for Cancel.
		handled, _, _ := syscall.SyscallN(w.api.isDialogMessage, w.hwnd, uintptr(unsafe.Pointer(&m)))
		if handled != 0 {
			continue
		}

		syscall.SyscallN(w.api.translateMessage, uintptr(unsafe.Pointer(&m)))
		syscall.SyscallN(w.api.dispatchMessage, uintptr(unsafe.Pointer(&m)))
	}
}

func (w *window) command(id int) {
	switch {
	case id == idOK:
		w.save()
	case id == idCancel:
		syscall.SyscallN(w.api.destroyWindow, w.hwnd)
	case id >= browseBase && id < browseBase+len(w.inputs):
		w.browse(w.inputs[id-browseBase])
	}
}

func (w *window) save() {
	if err := w.config.Save(w.read()); err != nil {
		w.tell(err)

		return
	}

	syscall.SyscallN(w.api.destroyWindow, w.hwnd)
}

func (w *window) read() []Section {
	edited := make([]Section, len(w.sections))
	for i, section := range w.sections {
		edited[i] = section
		edited[i].Fields = slices.Clone(section.Fields)
	}

	for _, in := range w.inputs {
		edited[in.section].Fields[in.field].Value = w.value(in)
	}

	return edited
}

func (w *window) value(in input) string {
	f := w.sections[in.section].Fields[in.field]
	if f.Kind != Choice {
		return w.text(in.hwnd)
	}

	picked, _, _ := syscall.SyscallN(w.api.sendMessage, in.hwnd, cbGetCurSel, 0, 0)
	if int32(picked) < 0 || int(picked) >= len(f.Options) {
		return f.Value
	}

	return f.Options[picked].Value
}

func (w *window) text(hwnd uintptr) string {
	length, _, _ := syscall.SyscallN(w.api.getWindowTextLength, hwnd)
	buffer := make([]uint16, length+1)
	syscall.SyscallN(w.api.getWindowText, hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))

	return syscall.UTF16ToString(buffer)
}

func (w *window) browse(in input) {
	f := w.sections[in.section].Fields[in.field]
	picked, err := w.choose(f.Kind, w.text(in.hwnd))
	if err != nil {
		w.tell(err)

		return
	}

	text, err := syscall.UTF16PtrFromString(picked)
	if picked == "" || err != nil {
		return
	}

	syscall.SyscallN(w.api.setWindowText, in.hwnd, uintptr(unsafe.Pointer(text)))
}

func (w *window) choose(kind Kind, current string) (string, error) {
	filter := utf16.Encode([]rune(filters[kind]))
	file := make([]uint16, fileUnits)
	request := openFileName{
		owner:   w.hwnd,
		filter:  &filter[0],
		file:    &file[0],
		maxFile: fileUnits,
		flags:   ofnExplorer | ofnFileMustExist | ofnPathMustExist | ofnHideReadOnly | ofnNoChangeDir,
	}
	request.structSize = uint32(unsafe.Sizeof(request))

	folder, err := syscall.UTF16PtrFromString(filepath.Dir(current))
	if filepath.IsAbs(current) && err == nil {
		request.initialDir = folder
	}

	picked, _, _ := syscall.SyscallN(w.api.openFile, uintptr(unsafe.Pointer(&request)))
	if picked != 0 {
		return syscall.UTF16ToString(file), nil
	}

	// GetOpenFileNameW answers the same for a failure and for Cancel; only the extended
	// error tells them apart, and it is zero for Cancel.
	code, _, _ := syscall.SyscallN(w.api.dialogError)
	if code == 0 {
		return "", nil
	}

	return "", fmt.Errorf("%w: GetOpenFileNameW: error %#x", ErrWindow, code)
}

func (w *window) tell(err error) {
	text, textErr := syscall.UTF16PtrFromString(err.Error())
	title, titleErr := syscall.UTF16PtrFromString("Sublayer")
	if textErr != nil || titleErr != nil {
		return
	}

	syscall.SyscallN(w.api.messageBox, w.hwnd, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbIconWarning)
}

func (w *window) release() {
	if w.font == 0 {
		return
	}

	syscall.SyscallN(w.api.deleteObject, w.font)
	w.font = 0
}

func (w *window) raise() {
	syscall.SyscallN(w.api.showWindow, w.hwnd, swRestore)
	syscall.SyscallN(w.api.setForegroundWindow, w.hwnd)
}

func (w *window) close() {
	// Windows may give the handle of a window that is gone to a window of another program.
	if w.gone() {
		return
	}

	syscall.SyscallN(w.api.postMessage, w.hwnd, wmClose, 0, 0)
	<-w.done
}

func (w *window) gone() bool {
	select {
	case <-w.done:
		return true
	default:
		return false
	}
}

func failure(name string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrWindow, name, err)
}
