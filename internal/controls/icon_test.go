package controls

import "testing"

type picture struct {
	pixels []byte
	size   int
}

func (p picture) at(x, y int) [4]byte {
	i := (y*p.size + x) * 4

	return [4]byte(p.pixels[i : i+4])
}

func TestIconIsADotOfTheColourOnAClearSquare(t *testing.T) {
	size := 32
	pixels := icon(size, [3]uint8{70, 200, 110})
	p := picture{pixels: pixels, size: size}

	if len(pixels) != size*size*4 {
		t.Fatalf("got %d bytes, want %d", len(pixels), size*size*4)
	}

	if got := p.at(size/2, size/2); got != [4]byte{110, 200, 70, 255} {
		t.Errorf("the middle is %v in BGRA, want the colour fully opaque", got)
	}

	for _, corner := range [][2]int{{0, 0}, {size - 1, 0}, {0, size - 1}, {size - 1, size - 1}} {
		if got := p.at(corner[0], corner[1]); got[3] != 0 {
			t.Errorf("the corner %v has alpha %d, want it clear", corner, got[3])
		}
	}
}

func TestIconRimIsDarkerThanItsMiddle(t *testing.T) {
	size := 32
	p := picture{pixels: icon(size, [3]uint8{70, 200, 110}), size: size}

	rim := p.at(size/2, 3)
	if rim[3] == 0 || rim[1] >= 200 {
		t.Errorf("the rim is %v, want a visible green darker than 200", rim)
	}
}

func TestIconSoftensItsEdge(t *testing.T) {
	size := 16
	pixels := icon(size, [3]uint8{240, 180, 40})

	soft := 0
	for i := 3; i < len(pixels); i += 4 {
		if pixels[i] > 0 && pixels[i] < 255 {
			soft++
		}
	}

	if soft == 0 {
		t.Error("every pixel is either clear or opaque, want a softened edge")
	}
}
