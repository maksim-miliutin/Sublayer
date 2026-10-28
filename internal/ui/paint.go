package ui

import "math"

const ringAlpha = 0.5

var ringColor = rgb{0, 0, 0}

// A layered window takes colours already multiplied by their alpha; a colour byte
// above its alpha byte shows as garbage, so every write goes through over.
func paint(mask []byte, s scene) []byte {
	out := make([]byte, 4*s.width*s.height)

	for _, p := range s.plates {
		shape(out, s, p.area, float64(s.radius), plateColor, p.alpha)
	}

	for _, b := range s.blocks {
		text(out, mask, s, b)
	}

	ring := rect{x: s.dot.x - 1, y: s.dot.y - 1, w: s.dot.w + 2, h: s.dot.h + 2}
	shape(out, s, ring, float64(ring.w)/2, ringColor, ringAlpha)
	shape(out, s, s.dot, float64(s.dot.w)/2, s.dotColor, 1)

	return out
}

func shape(out []byte, s scene, area rect, radius float64, c rgb, alpha float64) {
	for y := max(area.y, 0); y < min(area.bottom(), s.height); y++ {
		for x := max(area.x, 0); x < min(area.x+area.w, s.width); x++ {
			if cover := rounded(area, radius, float64(x)+0.5, float64(y)+0.5); cover > 0 {
				over(out, 4*(y*s.width+x), c, alpha*cover)
			}
		}
	}
}

func text(out, mask []byte, s scene, b block) {
	for y := max(b.area.y, 0); y < min(b.area.bottom(), s.height); y++ {
		for x := max(b.area.x, 0); x < min(b.area.x+b.area.w, s.width); x++ {
			i := 4 * (y*s.width + x)
			if cover := max(mask[i], mask[i+1], mask[i+2]); cover > 0 {
				over(out, i, b.color, b.alpha*float64(cover)/255)
			}
		}
	}
}

func rounded(area rect, radius, px, py float64) float64 {
	halfW, halfH := float64(area.w)/2, float64(area.h)/2
	qx := math.Abs(px-float64(area.x)-halfW) - (halfW - radius)
	qy := math.Abs(py-float64(area.y)-halfH) - (halfH - radius)
	distance := math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - radius

	return min(max(0.5-distance, 0), 1)
}

func over(out []byte, i int, c rgb, alpha float64) {
	keep := 1 - alpha

	out[i] = uint8(math.Round(float64(c.b)*alpha + float64(out[i])*keep))
	out[i+1] = uint8(math.Round(float64(c.g)*alpha + float64(out[i+1])*keep))
	out[i+2] = uint8(math.Round(float64(c.r)*alpha + float64(out[i+2])*keep))
	out[i+3] = uint8(math.Round(255*alpha + float64(out[i+3])*keep))
}
