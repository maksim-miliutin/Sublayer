package controls

import "math"

const rimShade = 0.55

type disc struct {
	middle, radius, rim float64
	colour              [3]float64 // blue, green, red, the order Windows keeps them in
}

// Windows wants the icon's colours as they are, not multiplied by their alpha.
func icon(size int, colour [3]uint8) []byte {
	d := disc{
		middle: float64(size) / 2,
		radius: float64(size) * 0.42,
		rim:    math.Max(1, float64(size)/16),
		colour: [3]float64{float64(colour[2]), float64(colour[1]), float64(colour[0])},
	}

	pixels := make([]byte, size*size*4)
	for y := range size {
		for x := range size {
			copy(pixels[(y*size+x)*4:], d.at(x, y))
		}
	}

	return pixels
}

func (d disc) at(x, y int) []byte {
	distance := math.Hypot(float64(x)+0.5-d.middle, float64(y)+0.5-d.middle)
	cover := math.Min(1, math.Max(0, d.radius-distance+0.5))
	if cover == 0 {
		return []byte{0, 0, 0, 0}
	}

	shade := 1.0
	if distance > d.radius-d.rim {
		shade = rimShade
	}

	return []byte{
		uint8(math.Round(d.colour[0] * shade)),
		uint8(math.Round(d.colour[1] * shade)),
		uint8(math.Round(d.colour[2] * shade)),
		uint8(math.Round(cover * 255)),
	}
}
