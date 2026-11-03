package controls

import (
	"testing"
	"unsafe"
)

func TestPanelStructsKeepTheLayoutWindowsExpects(t *testing.T) {
	word := unsafe.Sizeof(uintptr(0))

	want := map[string][2]uintptr{
		"WNDCLASSEXW":     {unsafe.Sizeof(wndClassEx{}), 4*4 + 8*word},
		"BITMAPV5HEADER":  {unsafe.Sizeof(bitmapV5Header{}), 124},
		"NOTIFYICONDATAW": {unsafe.Sizeof(notifyIconData{}), 956},
		"ICONINFO":        {unsafe.Sizeof(iconInfo{}), 20},
		"MSG":             {unsafe.Sizeof(msg{}), 28},
	}

	if word == 8 {
		want["NOTIFYICONDATAW"] = [2]uintptr{unsafe.Sizeof(notifyIconData{}), 976}
		want["ICONINFO"] = [2]uintptr{unsafe.Sizeof(iconInfo{}), 32}
		want["MSG"] = [2]uintptr{unsafe.Sizeof(msg{}), 48}
	}

	for name, sizes := range want {
		if sizes[0] != sizes[1] {
			t.Errorf("%s is %d bytes, want %d", name, sizes[0], sizes[1])
		}
	}
}
