package form

type box struct {
	x, y, w, h int
}

func (b box) right() int {
	return b.x + b.w
}

func (b box) bottom() int {
	return b.y + b.h
}

type row struct {
	label, input, browse box // browse stays empty for a field that takes no path
}

type plan struct {
	width, height int
	frames        []box   // one around each section, its title on the top edge
	rows          [][]row // one for each field of each section
	note          box
	save, cancel  box
}

type column struct {
	label, input int // pixels at 96 dpi; a path shares the input's width with its Browse button
}

var columns = []column{{label: 150, input: 380}, {label: 190, input: 70}}

type metrics struct {
	margin, gap, title, inset       int
	row, rowGap, labelGap           int
	browse, browseGap               int
	button, buttonHeight, buttonGap int
	columns                         []column
}

func scaled(dpi int) metrics {
	px := func(v int) int { return (v*dpi + 48) / 96 }

	m := metrics{
		margin: px(12), gap: px(10), title: px(22), inset: px(10), row: px(23), rowGap: px(6), labelGap: px(8),
		browseGap: px(6), browse: px(80), button: px(88), buttonHeight: px(26), buttonGap: px(8),
	}
	for _, c := range columns {
		m.columns = append(m.columns, column{label: px(c.label), input: px(c.input)})
	}

	return m
}

func (m metrics) width(c column) int {
	return m.inset + c.label + m.labelGap + c.input + m.inset
}

func arrange(sections []Section, dpi int) plan {
	m := scaled(dpi)

	lefts, tops := make([]int, len(m.columns)), make([]int, len(m.columns))
	x := m.margin
	for i, c := range m.columns {
		lefts[i], tops[i] = x, m.margin
		x += m.width(c) + m.margin
	}

	p := plan{width: x}
	lowest := m.margin
	for _, s := range sections {
		frame, rows := m.section(s, lefts[s.Column], tops[s.Column])
		p.frames = append(p.frames, frame)
		p.rows = append(p.rows, rows)
		tops[s.Column] = frame.bottom() + m.gap
		lowest = max(lowest, frame.bottom())
	}

	bar := lowest + m.margin
	p.cancel = box{x: p.width - m.margin - m.button, y: bar, w: m.button, h: m.buttonHeight}
	p.save = box{x: p.cancel.x - m.buttonGap - m.button, y: bar, w: m.button, h: m.buttonHeight}
	p.note = box{x: m.margin, y: bar, w: p.save.x - 2*m.margin, h: m.buttonHeight}
	p.height = bar + m.buttonHeight + m.margin

	return p
}

func (m metrics) section(s Section, left, top int) (box, []row) {
	c := m.columns[s.Column]
	labelX := left + m.inset
	inputX := labelX + c.label + m.labelGap

	rows := make([]row, len(s.Fields))
	y := top + m.title
	for i, f := range s.Fields {
		rows[i] = row{label: box{x: labelX, y: y, w: c.label, h: m.row}, input: box{x: inputX, y: y, w: c.input, h: m.row}}
		if f.Kind == Program || f.Kind == Model {
			rows[i].input.w = c.input - m.browseGap - m.browse
			rows[i].browse = box{x: rows[i].input.right() + m.browseGap, y: y, w: m.browse, h: m.row}
		}
		y += m.row + m.rowGap
	}

	return box{x: left, y: top, w: m.width(c), h: y - m.rowGap + m.inset - top}, rows
}
