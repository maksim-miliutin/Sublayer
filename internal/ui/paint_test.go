package ui

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func card() scene {
	return scene{
		width:    100,
		height:   60,
		plates:   []plate{{area: rect{0, 10, 100, 50}, alpha: 0.6}},
		blocks:   []block{{text: "x", translation: true, area: rect{10, 20, 80, 20}, color: translationColor, alpha: 1}},
		dot:      rect{86, 0, 10, 10},
		dotColor: stateColor(Listening),
		radius:   8,
	}
}

func blank(s scene) pixels {
	return pixels{bytes: make([]byte, 4*s.width*s.height), width: s.width, height: s.height}
}

func painted(s scene, mask pixels) pixels {
	return pixels{bytes: paint(mask.bytes, s), width: s.width, height: s.height}
}

// Offsets are counted here rather than through at, so a test can catch at going wrong:
// rows from the top, four bytes a pixel, as a top-down DIB lays them out.
func offset(p pixels, x, y int) int {
	return (y*p.width + x) * 4
}

func (p pixels) set(x, y int, level byte) {
	i := offset(p, x, y)
	p.bytes[i], p.bytes[i+1], p.bytes[i+2] = level, level, level
}

func (p pixels) px(x, y int) []byte {
	i := offset(p, x, y)

	return p.bytes[i : i+4]
}

func TestPaintLeavesEverythingOffThePlatesClear(t *testing.T) {
	s := card()

	out := painted(s, blank(s))

	if got := out.px(5, 5); !slices.Equal(got, []byte{0, 0, 0, 0}) {
		t.Errorf("a pixel off every plate is % x, want fully clear", got)
	}

	if got := out.px(50, 9); !slices.Equal(got, []byte{0, 0, 0, 0}) {
		t.Errorf("the pixel just above the plate is % x, want the plate not to bleed out", got)
	}
}

func TestPaintDarkensThePlateToItsAlpha(t *testing.T) {
	s := card()

	if got := painted(s, blank(s)).px(50, 50); !slices.Equal(got, []byte{12, 10, 10, 153}) {
		t.Errorf("plate interior is % x, want 0c 0a 0a 99", got)
	}
}

func TestPaintLaysCoveredTextOpaqueOverThePlate(t *testing.T) {
	s := card()
	mask := blank(s)
	mask.set(30, 25, 255)

	if got := painted(s, mask).px(30, 25); !slices.Equal(got, []byte{255, 255, 255, 255}) {
		t.Errorf("fully covered text is % x, want opaque white", got)
	}
}

func TestPaintFadesTextWithItsPair(t *testing.T) {
	s := card()
	s.blocks[0].alpha = 0.5
	mask := blank(s)
	mask.set(30, 25, 255)

	if got := painted(s, mask).px(30, 25); !slices.Equal(got, []byte{134, 133, 133, 204}) {
		t.Errorf("half-faded text over the plate is % x, want 86 85 85 cc", got)
	}
}

func TestPaintIgnoresCoverageOutsideTheBlocks(t *testing.T) {
	s := card()
	mask := blank(s)
	mask.set(50, 50, 255)

	if got := painted(s, mask).px(50, 50); !slices.Equal(got, []byte{12, 10, 10, 153}) {
		t.Errorf("stray coverage off the text turned the plate into % x", got)
	}
}

func TestPaintRoundsThePlateCorners(t *testing.T) {
	s := card()
	out := painted(s, blank(s))

	if got := out.px(0, 10); got[3] != 0 {
		t.Errorf("the plate corner has alpha %d, want it cut away", got[3])
	}

	if got := out.px(0, 35); !slices.Equal(got, []byte{12, 10, 10, 153}) {
		t.Errorf("the middle of the left edge is % x, want the full plate", got)
	}
}

func TestPaintSoftensTheEdgeOfARoundedCorner(t *testing.T) {
	s := card()

	if got := painted(s, blank(s)).px(2, 12); got[3] == 0 || got[3] >= 153 {
		t.Errorf("a pixel the corner arc runs through has alpha %d, want between 0 and the plate's 153", got[3])
	}
}

func TestPaintFillsASquarePlateRightUpToItsCorners(t *testing.T) {
	s := card()
	s.radius = 0
	out := painted(s, blank(s))

	for _, at := range [][2]int{{0, 10}, {99, 59}, {50, 50}} {
		if got := out.px(at[0], at[1]); !slices.Equal(got, []byte{12, 10, 10, 153}) {
			t.Errorf("square plate pixel %v is % x, want the full plate", at, got)
		}
	}
}

func TestPaintDrawsTheStateDotInItsColour(t *testing.T) {
	s := card()
	c := stateColor(Listening)

	if got := painted(s, blank(s)).px(90, 4); !slices.Equal(got, []byte{c.b, c.g, c.r, 255}) {
		t.Errorf("the dot centre is % x, want the listening colour", got)
	}
}

func TestPaintKeepsEveryPixelPremultiplied(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 4))
	s := card()
	s.blocks[0].alpha = 0.7
	mask := blank(s)
	for i := range mask.bytes {
		mask.bytes[i] = byte(random.IntN(256))
	}

	out := painted(s, mask).bytes
	for i := 0; i < len(out); i += 4 {
		if out[i] > out[i+3] || out[i+1] > out[i+3] || out[i+2] > out[i+3] {
			t.Fatalf("pixel %d is % x, a colour above its alpha", i/4, out[i:i+4])
		}
	}
}
