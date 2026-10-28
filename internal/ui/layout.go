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
	pad, gap, line, radius, dot, margin int // pixels at the screen's density
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

func (m metrics) plateHeight(h heights) int {
	inner := h.original + h.translation
	if h.original > 0 && h.translation > 0 {
		inner += m.line
	}

	return m.pad + inner + m.pad
}

type layout struct {
	metrics
	work      rect
	width     int
	maxHeight int
}

func layoutFor(work rect, m metrics) layout {
	return layout{
		metrics:   m,
		work:      work,
		width:     int(float64(work.w) * widthShare),
		maxHeight: work.h - 2*m.margin,
	}
}

func (l layout) textWidth() int {
	return l.width - 2*l.pad
}

func (l layout) fitting(measured []heights) int {
	total := l.dot
	for i := len(measured) - 1; i >= 0; i-- {
		total += l.gap + l.plateHeight(measured[i])
		if total > l.maxHeight && i < len(measured)-1 {
			return i + 1
		}
	}

	return 0
}

func (l layout) arrange(f Frame, measured []heights) scene {
	s := scene{
		width:    l.width,
		dot:      rect{x: l.width - l.pad - l.dot, y: 0, w: l.dot, h: l.dot},
		dotColor: stateColor(f.State),
		radius:   l.radius,
	}

	y := l.dot
	for i := l.fitting(measured); i < len(f.Pairs); i++ {
		alpha := fade(len(f.Pairs) - 1 - i)
		h := measured[i]
		y += l.gap

		area := rect{x: 0, y: y, w: l.width, h: l.plateHeight(h)}
		s.plates = append(s.plates, plate{area: area, alpha: plateAlpha})

		inner := rect{x: l.pad, y: y + l.pad, w: l.textWidth()}
		if h.original > 0 {
			inner.h = h.original
			s.blocks = append(s.blocks, block{text: f.Pairs[i].Original, area: inner, color: originalColor, alpha: alpha})
			inner.y += h.original + l.line
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

func (l layout) place(height int) rect {
	return rect{x: l.work.x + l.margin, y: l.work.bottom() - l.margin - height, w: l.width, h: height}
}
