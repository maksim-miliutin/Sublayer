package ui

import "math"

const ringAlpha = 0.5

var ringColor = rgb{0, 0, 0}

type pixels struct {
	bytes         []byte
	width, height int
}

type shape struct {
	area   rect
	radius float64
	color  rgb
	alpha  float64
}

// A layered window takes colours already multiplied by their alpha; a colour byte
// above its alpha byte shows as garbage, so every write goes through over.
func paint(mask []byte, s scene) []byte {
	out := pixels{bytes: make([]byte, 4*s.width*s.height), width: s.width, height: s.height}
	ink := pixels{bytes: mask, width: s.width, height: s.height}

	for _, p := range s.plates {
		out.fill(shape{area: p.area, radius: float64(s.radius), color: plateColor, alpha: p.alpha})
	}

	for _, b := range s.blocks {
		out.write(ink, b)
	}

	ring := rect{x: s.dot.x - 1, y: s.dot.y - 1, w: s.dot.w + 2, h: s.dot.h + 2}
	out.fill(shape{area: ring, radius: float64(ring.w) / 2, color: ringColor, alpha: ringAlpha})
	out.fill(shape{area: s.dot, radius: float64(s.dot.w) / 2, color: s.dotColor, alpha: 1})

	return out.bytes
}

func (p pixels) at(x, y int) int {
	return 4 * (y*p.width + x)
}

func (p pixels) fill(sh shape) {
	for y := max(sh.area.y, 0); y < min(sh.area.bottom(), p.height); y++ {
		for x := max(sh.area.x, 0); x < min(sh.area.x+sh.area.w, p.width); x++ {
			p.over(p.at(x, y), sh.color, sh.alpha*sh.cover(float64(x)+0.5, float64(y)+0.5))
		}
	}
}

func (p pixels) write(ink pixels, b block) {
	for y := max(b.area.y, 0); y < min(b.area.bottom(), p.height); y++ {
		for x := max(b.area.x, 0); x < min(b.area.x+b.area.w, p.width); x++ {
			i := p.at(x, y)
			p.over(i, b.color, b.alpha*ink.coverage(i))
		}
	}
}

func (p pixels) coverage(i int) float64 {
	return float64(max(p.bytes[i], p.bytes[i+1], p.bytes[i+2])) / 255
}

func (p pixels) over(i int, c rgb, alpha float64) {
	keep := 1 - alpha

	p.bytes[i] = uint8(math.Round(float64(c.b)*alpha + float64(p.bytes[i])*keep))
	p.bytes[i+1] = uint8(math.Round(float64(c.g)*alpha + float64(p.bytes[i+1])*keep))
	p.bytes[i+2] = uint8(math.Round(float64(c.r)*alpha + float64(p.bytes[i+2])*keep))
	p.bytes[i+3] = uint8(math.Round(255*alpha + float64(p.bytes[i+3])*keep))
}

func (sh shape) cover(px, py float64) float64 {
	halfW, halfH := float64(sh.area.w)/2, float64(sh.area.h)/2
	qx := math.Abs(px-float64(sh.area.x)-halfW) - (halfW - sh.radius)
	qy := math.Abs(py-float64(sh.area.y)-halfH) - (halfH - sh.radius)
	distance := math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - sh.radius

	return min(max(0.5-distance, 0), 1)
}
