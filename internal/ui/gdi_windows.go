package ui

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/maksim-miliutin/Sublayer/internal/winapi"
)

const (
	dtWordBreak        = 0x10
	dtCalcRect         = 0x400
	dtNoPrefix         = 0x800
	bkTransparent      = 1
	antialiasedQuality = 4
	defaultCharset     = 1
	fwNormal           = 400
	fwSemibold         = 600
	biRGB              = 0
	dibRGBColors       = 0
	white              = 0x00ffffff
	wholeString        = ^uintptr(0)

	originalPoints    = 11
	translationPoints = 15
	fontFace          = "Segoe UI"
)

type winRect struct {
	left, top, right, bottom int32
}

type bitmapInfo struct {
	size        uint32
	width       int32
	height      int32
	planes      uint16
	bits        uint16
	compression uint32
	imageSize   uint32
	xPerMeter   int32
	yPerMeter   int32
	used        uint32
	important   uint32
	colors      [1]uint32
}

type gdi struct {
	createCompatibleDC uintptr
	deleteDC           uintptr
	createDIBSection   uintptr
	selectObject       uintptr
	deleteObject       uintptr
	createFont         uintptr
	setTextColor       uintptr
	setBkMode          uintptr
	gdiFlush           uintptr
	drawText           uintptr
}

var loadGDI = sync.OnceValues(func() (*gdi, error) {
	g := &gdi{}

	err := winapi.Load("gdi32.dll",
		winapi.Proc{Name: "CreateCompatibleDC", Addr: &g.createCompatibleDC},
		winapi.Proc{Name: "DeleteDC", Addr: &g.deleteDC},
		winapi.Proc{Name: "CreateDIBSection", Addr: &g.createDIBSection},
		winapi.Proc{Name: "SelectObject", Addr: &g.selectObject},
		winapi.Proc{Name: "DeleteObject", Addr: &g.deleteObject},
		winapi.Proc{Name: "CreateFontW", Addr: &g.createFont},
		winapi.Proc{Name: "SetTextColor", Addr: &g.setTextColor},
		winapi.Proc{Name: "SetBkMode", Addr: &g.setBkMode},
		winapi.Proc{Name: "GdiFlush", Addr: &g.gdiFlush},
	)
	if err != nil {
		return nil, err
	}

	if err := winapi.Load("user32.dll", winapi.Proc{Name: "DrawTextW", Addr: &g.drawText}); err != nil {
		return nil, err
	}

	return g, nil
})

func failure(name string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrWindow, name, err)
}

type renderer struct {
	g           *gdi
	m           metrics
	original    uintptr
	translation uintptr
}

func newRenderer(dpi int) (*renderer, error) {
	g, err := loadGDI()
	if err != nil {
		return nil, err
	}

	r := &renderer{g: g, m: scaledMetrics(dpi)}

	r.original, err = g.font(originalPoints*dpi/72, fwNormal)
	if err != nil {
		return nil, err
	}

	r.translation, err = g.font(translationPoints*dpi/72, fwSemibold)
	if err != nil {
		r.close()
		return nil, err
	}

	return r, nil
}

func (g *gdi) font(pixels, weight int) (uintptr, error) {
	face, err := syscall.UTF16PtrFromString(fontFace)
	if err != nil {
		return 0, err
	}

	height := -pixels
	f, _, callErr := syscall.SyscallN(g.createFont, uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0,
		defaultCharset, 0, 0, antialiasedQuality, 0, uintptr(unsafe.Pointer(face)))
	if f == 0 {
		return 0, failure("CreateFontW", callErr)
	}

	return f, nil
}

func (r *renderer) close() {
	for _, f := range []uintptr{r.original, r.translation} {
		if f != 0 {
			syscall.SyscallN(r.g.deleteObject, f)
		}
	}
}

type canvas struct {
	g      *gdi
	dc     uintptr
	bitmap uintptr
	old    uintptr
	pixels []byte
	scene  scene
}

func (r *renderer) draw(f Frame, width, maxHeight int) (*canvas, error) {
	dc, _, callErr := syscall.SyscallN(r.g.createCompatibleDC, 0)
	if dc == 0 {
		return nil, failure("CreateCompatibleDC", callErr)
	}

	c := &canvas{g: r.g, dc: dc}

	measured, err := r.measure(dc, f, width-2*r.m.pad)
	if err != nil {
		c.free()
		return nil, err
	}

	c.scene = arrange(f, measured, width, maxHeight, r.m)
	if err := c.allocate(); err != nil {
		c.free()
		return nil, err
	}

	if err := r.write(c); err != nil {
		c.free()
		return nil, err
	}

	// GDI batches drawing; the bits hold the text only once the batch is flushed.
	syscall.SyscallN(r.g.gdiFlush)
	copy(c.pixels, paint(c.pixels, c.scene))

	return c, nil
}

func (r *renderer) measure(dc uintptr, f Frame, width int) ([]heights, error) {
	out := make([]heights, len(f.Pairs))
	for i, p := range f.Pairs {
		var err error

		out[i].original, err = r.height(dc, r.original, p.Original, width)
		if err != nil {
			return nil, err
		}

		out[i].translation, err = r.height(dc, r.translation, p.Translation, width)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (r *renderer) height(dc, font uintptr, text string, width int) (int, error) {
	if text == "" {
		return 0, nil
	}

	chars, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return 0, failure("text", err)
	}

	syscall.SyscallN(r.g.selectObject, dc, font)

	area := winRect{right: int32(width)}
	h, _, callErr := syscall.SyscallN(r.g.drawText, dc, uintptr(unsafe.Pointer(chars)), wholeString,
		uintptr(unsafe.Pointer(&area)), dtCalcRect|dtWordBreak|dtNoPrefix)
	if h == 0 {
		return 0, failure("DrawTextW", callErr)
	}

	return int(area.bottom), nil
}

// A negative height asks for rows from the top down, the order the scene is laid out in.
func (c *canvas) allocate() error {
	info := bitmapInfo{
		size:   40,
		width:  int32(c.scene.width),
		height: -int32(c.scene.height),
		planes: 1,
		bits:   32,
	}

	var bits unsafe.Pointer
	bitmap, _, callErr := syscall.SyscallN(c.g.createDIBSection, c.dc, uintptr(unsafe.Pointer(&info)),
		dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 {
		return failure("CreateDIBSection", callErr)
	}

	c.bitmap = bitmap
	c.old, _, _ = syscall.SyscallN(c.g.selectObject, c.dc, bitmap)
	c.pixels = unsafe.Slice((*byte)(bits), 4*c.scene.width*c.scene.height)
	clear(c.pixels)

	return nil
}

func (r *renderer) write(c *canvas) error {
	syscall.SyscallN(r.g.setBkMode, c.dc, bkTransparent)
	syscall.SyscallN(r.g.setTextColor, c.dc, white)

	for _, b := range c.scene.blocks {
		font := r.original
		if b.translation {
			font = r.translation
		}

		chars, err := syscall.UTF16PtrFromString(b.text)
		if err != nil {
			return failure("text", err)
		}

		syscall.SyscallN(r.g.selectObject, c.dc, font)

		area := winRect{left: int32(b.area.x), top: int32(b.area.y), right: int32(b.area.x + b.area.w), bottom: int32(b.area.bottom())}
		h, _, callErr := syscall.SyscallN(r.g.drawText, c.dc, uintptr(unsafe.Pointer(chars)), wholeString,
			uintptr(unsafe.Pointer(&area)), dtWordBreak|dtNoPrefix)
		if h == 0 {
			return failure("DrawTextW", callErr)
		}
	}

	return nil
}

func (c *canvas) free() {
	if c.bitmap != 0 {
		syscall.SyscallN(c.g.selectObject, c.dc, c.old)
		syscall.SyscallN(c.g.deleteObject, c.bitmap)
	}

	syscall.SyscallN(c.g.deleteDC, c.dc)
}
