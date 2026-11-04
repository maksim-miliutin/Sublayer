package form

import (
	"testing"
	"unsafe"
)

func TestStructsKeepTheLayoutWindowsExpects(t *testing.T) {
	wide := unsafe.Sizeof(uintptr(0)) == 8

	cases := []struct {
		name         string
		got          uintptr
		wide, narrow uintptr
	}{
		{"OPENFILENAMEW", unsafe.Sizeof(openFileName{}), 152, 88},
		{"WNDCLASSEXW", unsafe.Sizeof(wndClassEx{}), 80, 48},
		{"MSG", unsafe.Sizeof(msg{}), 48, 28},
		{"RECT", unsafe.Sizeof(winRect{}), 16, 16},
	}
	for _, c := range cases {
		want := c.narrow
		if wide {
			want = c.wide
		}

		if c.got != want {
			t.Errorf("%s is %d bytes, want %d", c.name, c.got, want)
		}
	}
}

func TestTheSystemHasEveryFunctionTheWindowCalls(t *testing.T) {
	if _, err := loadAPI(); err != nil {
		t.Error(err)
	}
}

func TestEveryPathFieldHasAFilterForItsFiles(t *testing.T) {
	for _, f := range flatten(sections) {
		filter, found := filters[f.Kind]
		if (f.Kind == Program || f.Kind == Model) && (!found || filter[len(filter)-2:] != "\x00\x00") {
			t.Errorf("%s takes a path but has no filter ending in two zeros: %q", f.Label, filter)
		}
	}
}
