package ui

import (
	"testing"
	"unsafe"
)

func TestWindowStructsKeepTheLayoutWindowsExpects(t *testing.T) {
	word := unsafe.Sizeof(uintptr(0))

	want := map[string][2]uintptr{
		"WNDCLASSEXW":   {unsafe.Sizeof(wndClassEx{}), 4*4 + 8*word},
		"POINT":         {unsafe.Sizeof(point{}), 8},
		"SIZE":          {unsafe.Sizeof(size{}), 8},
		"BLENDFUNCTION": {unsafe.Sizeof(blendFunction{}), 4},
	}

	if word == 8 {
		want["MSG"] = [2]uintptr{unsafe.Sizeof(msg{}), 48}
	} else {
		want["MSG"] = [2]uintptr{unsafe.Sizeof(msg{}), 28}
	}

	for name, sizes := range want {
		if sizes[0] != sizes[1] {
			t.Errorf("%s is %d bytes, want %d", name, sizes[0], sizes[1])
		}
	}
}
