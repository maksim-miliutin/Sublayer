//go:build windows && (amd64 || arm64)

package capture

import (
	"encoding/binary"
	"fmt"
	"slices"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/maksim-miliutin/Sublayer/internal/winapi"
)

type guid struct {
	a uint32
	b uint16
	c uint16
	d [8]byte
}

var (
	clsidEnumerator  = guid{0xbcde0395, 0xe52f, 0x467c, [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}}
	iidEnumerator    = guid{0xa95664d2, 0x9614, 0x4f35, [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}}
	iidAudioClient   = guid{0x1cb9ad4c, 0xdbfa, 0x4c32, [8]byte{0xb1, 0x78, 0xc2, 0xf5, 0x68, 0xa7, 0x03, 0xb2}}
	iidCaptureClient = guid{0xc8adbd64, 0xe71e, 0x48a0, [8]byte{0xa4, 0xde, 0x18, 0x5c, 0x39, 0x5c, 0xd3, 0x17}}
)

const (
	coinitMultithreaded = 0x0
	clsctxAll           = 0x17
	eRender             = 0
	eConsole            = 0
	shareModeShared     = 0
	streamLoopback      = 0x00020000
	bufferSilent        = 0x2

	bufferSpan = 10_000_000 // 100-nanosecond units: one second of room in the device buffer
)

type method struct {
	index uintptr
	name  string
}

var (
	unknownRelease    = method{2, "Release"}
	enumeratorDefault = method{4, "GetDefaultAudioEndpoint"}
	deviceActivate    = method{3, "Activate"}
	clientInitialize  = method{3, "Initialize"}
	clientMixFormat   = method{8, "GetMixFormat"}
	clientStart       = method{10, "Start"}
	clientStop        = method{11, "Stop"}
	clientService     = method{14, "GetService"}
	captureBuffer     = method{3, "GetBuffer"}
	captureRelease    = method{4, "ReleaseBuffer"}
	captureNextSize   = method{5, "GetNextPacketSize"}
)

type ole struct {
	initialize     uintptr
	uninitialize   uintptr
	createInstance uintptr
	taskMemFree    uintptr
}

var loadOle = sync.OnceValues(findOle)

func Open() (Source, error) {
	return startStream(&wasapi{}, pollEvery)
}

func findOle() (ole, error) {
	var o ole

	err := winapi.Load("ole32.dll",
		winapi.Proc{Name: "CoInitializeEx", Addr: &o.initialize},
		winapi.Proc{Name: "CoUninitialize", Addr: &o.uninitialize},
		winapi.Proc{Name: "CoCreateInstance", Addr: &o.createInstance},
		winapi.Proc{Name: "CoTaskMemFree", Addr: &o.taskMemFree},
	)
	if err != nil {
		return ole{}, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}

	return o, nil
}

func slot(object unsafe.Pointer, m method) uintptr {
	table := *(*unsafe.Pointer)(object)

	return *(*uintptr)(unsafe.Add(table, m.index*unsafe.Sizeof(uintptr(0))))
}

func failed(name string, hr uintptr) error {
	if int32(hr) >= 0 {
		return nil
	}

	return fmt.Errorf("%w: %s returned %#08x", ErrDevice, name, uint32(hr))
}

type wasapi struct {
	ole        ole
	comReady   bool
	enumerator unsafe.Pointer
	endpoint   unsafe.Pointer
	client     unsafe.Pointer
	capture    unsafe.Pointer
	started    bool

	format    format
	resampler *resampler
	gaps      *filler
}

func (w *wasapi) open() error {
	o, err := loadOle()
	if err != nil {
		return err
	}
	w.ole = o

	hr, _, _ := syscall.SyscallN(o.initialize, 0, coinitMultithreaded)
	if err := failed("CoInitializeEx", hr); err != nil {
		return err
	}
	w.comReady = true

	hr, _, _ = syscall.SyscallN(o.createInstance, uintptr(unsafe.Pointer(&clsidEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidEnumerator)), uintptr(unsafe.Pointer(&w.enumerator)))
	if err := failed("CoCreateInstance", hr); err != nil {
		return err
	}

	hr, _, _ = syscall.SyscallN(slot(w.enumerator, enumeratorDefault), uintptr(w.enumerator),
		eRender, eConsole, uintptr(unsafe.Pointer(&w.endpoint)))
	if err := failed(enumeratorDefault.name, hr); err != nil {
		return err
	}

	hr, _, _ = syscall.SyscallN(slot(w.endpoint, deviceActivate), uintptr(w.endpoint),
		uintptr(unsafe.Pointer(&iidAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&w.client)))
	if err := failed(deviceActivate.name, hr); err != nil {
		return err
	}

	return w.startLoopback()
}

func (w *wasapi) startLoopback() error {
	var mix unsafe.Pointer
	hr, _, _ := syscall.SyscallN(slot(w.client, clientMixFormat), uintptr(w.client), uintptr(unsafe.Pointer(&mix)))
	if err := failed(clientMixFormat.name, hr); err != nil {
		return err
	}
	defer syscall.SyscallN(w.ole.taskMemFree, uintptr(mix))

	f, err := parseFormat(formatBytes(mix))
	if err != nil {
		return err
	}

	// REFERENCE_TIME is 64 bits wide and fits one argument register only on a 64-bit
	// build, which is why this file is built for amd64 and arm64 alone.
	hr, _, _ = syscall.SyscallN(slot(w.client, clientInitialize), uintptr(w.client),
		shareModeShared, streamLoopback, bufferSpan, 0, uintptr(mix), 0)
	if err := failed(clientInitialize.name, hr); err != nil {
		return err
	}

	hr, _, _ = syscall.SyscallN(slot(w.client, clientService), uintptr(w.client),
		uintptr(unsafe.Pointer(&iidCaptureClient)), uintptr(unsafe.Pointer(&w.capture)))
	if err := failed(clientService.name, hr); err != nil {
		return err
	}

	hr, _, _ = syscall.SyscallN(slot(w.client, clientStart), uintptr(w.client))
	if err := failed(clientStart.name, hr); err != nil {
		return err
	}
	w.started = true

	w.format = f
	w.resampler = newResampler(f.rate, Rate)
	w.gaps = newFiller(f.rate, time.Now())

	return nil
}

func formatBytes(mix unsafe.Pointer) []byte {
	head := unsafe.Slice((*byte)(mix), plainLen)
	extra := int(binary.LittleEndian.Uint16(head[16:]))

	return slices.Clone(unsafe.Slice((*byte)(mix), plainLen+extra))
}

func (w *wasapi) drain(now time.Time) ([]float32, error) {
	var heard []float32
	for {
		var frames uint32
		hr, _, _ := syscall.SyscallN(slot(w.capture, captureNextSize), uintptr(w.capture), uintptr(unsafe.Pointer(&frames)))
		if err := failed(captureNextSize.name, hr); err != nil {
			return nil, err
		}

		if frames == 0 {
			break
		}

		var packet unsafe.Pointer
		var flags uint32
		hr, _, _ = syscall.SyscallN(slot(w.capture, captureBuffer), uintptr(w.capture),
			uintptr(unsafe.Pointer(&packet)), uintptr(unsafe.Pointer(&frames)), uintptr(unsafe.Pointer(&flags)), 0, 0)
		if err := failed(captureBuffer.name, hr); err != nil {
			return nil, err
		}

		heard = append(heard, w.decode(packet, frames, flags)...)

		hr, _, _ = syscall.SyscallN(slot(w.capture, captureRelease), uintptr(w.capture), uintptr(frames))
		if err := failed(captureRelease.name, hr); err != nil {
			return nil, err
		}
	}

	if len(heard) > 0 {
		w.gaps.hear(now)

		return w.resampler.feed(heard), nil
	}

	return w.resampler.feed(make([]float32, w.gaps.pay(now))), nil
}

// Windows marks a packet silent and leaves its bytes undefined, so they are never read.
func (w *wasapi) decode(packet unsafe.Pointer, frames, flags uint32) []float32 {
	if flags&bufferSilent != 0 {
		return make([]float32, frames)
	}

	return w.format.mono(unsafe.Slice((*byte)(packet), int(frames)*w.format.block))
}

func (w *wasapi) close() {
	if w.started {
		syscall.SyscallN(slot(w.client, clientStop), uintptr(w.client))
	}

	for _, object := range []unsafe.Pointer{w.capture, w.client, w.endpoint, w.enumerator} {
		if object != nil {
			syscall.SyscallN(slot(object, unknownRelease), uintptr(object))
		}
	}

	if w.comReady {
		syscall.SyscallN(w.ole.uninitialize)
	}
}
