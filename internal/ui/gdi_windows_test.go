package ui

import (
	"strings"
	"testing"
	"unsafe"
)

func rendered(t *testing.T, f Frame, width int) *canvas {
	t.Helper()

	r, err := newRenderer(96)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.close)

	c, err := r.draw(f, width, 2000)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.free)

	return c
}

func brightIn(c *canvas, area rect) int {
	var n int
	for y := area.y; y < area.bottom(); y++ {
		for x := area.x; x < area.x+area.w; x++ {
			i := 4 * (y*c.scene.width + x)
			if c.pixels[i+2] >= 200 && c.pixels[i+3] >= 200 {
				n++
			}
		}
	}

	return n
}

func TestStructsKeepTheLayoutWindowsExpects(t *testing.T) {
	if n := unsafe.Sizeof(bitmapInfo{}); n != 44 {
		t.Errorf("bitmapInfo is %d bytes, want 44", n)
	}

	if n := unsafe.Sizeof(winRect{}); n != 16 {
		t.Errorf("winRect is %d bytes, want 16", n)
	}
}

func TestRendererWrapsLongTextOntoMoreLines(t *testing.T) {
	short := rendered(t, Frame{Pairs: []Pair{{Original: "Oui.", Translation: "Да."}}}, 320)
	long := rendered(t, Frame{Pairs: []Pair{{Original: "Oui.", Translation: strings.Repeat("Здравствуйте, спасибо, что пришли. ", 6)}}}, 320)

	one, many := short.scene.blocks[1].area.h, long.scene.blocks[1].area.h
	if one == 0 || many < 3*one {
		t.Errorf("a line is %d px high and six sentences in 320 px came to %d, want at least three lines", one, many)
	}
}

func TestRendererDrawsCyrillicTextWhereItsBlockIs(t *testing.T) {
	c := rendered(t, Frame{Pairs: []Pair{{Original: "Bonjour", Translation: "Здравствуйте"}}}, 400)

	translation := c.scene.blocks[1]
	if n := brightIn(c, translation.area); n < 30 {
		t.Errorf("only %d bright opaque pixels inside the translation block %+v", n, translation.area)
	}

	below := rect{x: 0, y: translation.area.bottom() + 1, w: c.scene.width, h: c.scene.height - translation.area.bottom() - 1}
	if n := brightIn(c, below); n != 0 {
		t.Errorf("%d bright pixels below the last block, where only the plate should be", n)
	}
}

func TestRendererLeavesTheTopLeftCornerSeeThrough(t *testing.T) {
	c := rendered(t, Frame{Pairs: []Pair{{Original: "a", Translation: "b"}}}, 400)

	if a := c.pixels[3]; a != 0 {
		t.Errorf("the top-left pixel has alpha %d, want the window clear there", a)
	}
}
