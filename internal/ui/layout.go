package ui

type rect struct {
	x, y, w, h int
}

func (r rect) bottom() int {
	return r.y + r.h
}

type rgb struct {
	r, g, b uint8
}

type metrics struct {
	pad, gap, line, radius, dot, margin int
}

func scaledMetrics(dpi int) metrics {
	px := func(v int) int { return (v*dpi + 48) / 96 }

	return metrics{pad: px(10), gap: px(6), line: px(2), radius: px(8), dot: px(10), margin: px(16)}
}

type heights struct {
	original, translation int
}

type block struct {
	text        string
	translation bool
	area        rect
	color       rgb
	alpha       float64
}

type plate struct {
	area  rect
	alpha float64
}

type scene struct {
	width, height int
	plates        []plate
	blocks        []block
	dot           rect
	dotColor      rgb
	radius        int
}

const (
	widthShare = 0.3
	plateAlpha = 0.85
	fadeStep   = 0.15
	fadeFloor  = 0.45
)

var (
	originalColor    = rgb{175, 175, 180}
	translationColor = rgb{255, 255, 255}
	plateColor       = rgb{16, 16, 20}
)

var stateColors = map[State]rgb{
	Listening:   {70, 200, 110},
	Recognizing: {240, 180, 40},
	Paused:      {150, 150, 150},
	Failed:      {230, 70, 60},
}

func stateColor(s State) rgb {
	c, known := stateColors[s]
	if !known {
		return stateColors[Failed]
	}

	return c
}

func fade(age int) float64 {
	return max(fadeFloor, 1-fadeStep*float64(age))
}

func plateHeight(h heights, m metrics) int {
	inner := h.original + h.translation
	if h.original > 0 && h.translation > 0 {
		inner += m.line
	}

	return m.pad + inner + m.pad
}

func fitting(measured []heights, maxHeight int, m metrics) int {
	total := m.dot
	for i := len(measured) - 1; i >= 0; i-- {
		total += m.gap + plateHeight(measured[i], m)
		if total > maxHeight && i < len(measured)-1 {
			return i + 1
		}
	}

	return 0
}

func arrange(f Frame, measured []heights, width, maxHeight int, m metrics) scene {
	s := scene{
		width:    width,
		dot:      rect{x: width - m.pad - m.dot, y: 0, w: m.dot, h: m.dot},
		dotColor: stateColor(f.State),
		radius:   m.radius,
	}

	y := m.dot
	for i := fitting(measured, maxHeight, m); i < len(f.Pairs); i++ {
		alpha := fade(len(f.Pairs) - 1 - i)
		h := measured[i]
		y += m.gap

		area := rect{x: 0, y: y, w: width, h: plateHeight(h, m)}
		s.plates = append(s.plates, plate{area: area, alpha: plateAlpha})

		inner := rect{x: m.pad, y: y + m.pad, w: width - 2*m.pad}
		if h.original > 0 {
			inner.h = h.original
			s.blocks = append(s.blocks, block{text: f.Pairs[i].Original, area: inner, color: originalColor, alpha: alpha})
			inner.y += h.original + m.line
		}

		if h.translation > 0 {
			inner.h = h.translation
			s.blocks = append(s.blocks, block{text: f.Pairs[i].Translation, translation: true, area: inner, color: translationColor, alpha: alpha})
		}

		y = area.bottom()
	}

	s.height = y

	return s
}

func widthFor(work rect) int {
	return int(float64(work.w) * widthShare)
}

func place(work rect, width, height int, m metrics) rect {
	return rect{x: work.x + m.margin, y: work.bottom() - m.margin - height, w: width, h: height}
}
