package form

import "testing"

var densities = []int{96, 120, 144, 192}

type spot struct {
	section int
	field   Field
	row     row
}

func spots(p plan) []spot {
	var all []spot
	for i, rows := range p.rows {
		for j, r := range rows {
			all = append(all, spot{section: i, field: sections[i].Fields[j], row: r})
		}
	}

	return all
}

func (r row) boxes() []box {
	if r.browse == (box{}) {
		return []box{r.label, r.input}
	}

	return []box{r.label, r.input, r.browse}
}

func controlsOf(p plan) []box {
	boxes := []box{p.note, p.save, p.cancel}
	for _, s := range spots(p) {
		boxes = append(boxes, s.row.boxes()...)
	}

	return boxes
}

func misplaced(boxes []box, area box) []box {
	var out []box
	for _, b := range boxes {
		if b.w <= 0 || b.h <= 0 || b.x < area.x || b.y < area.y || b.right() > area.right() || b.bottom() > area.bottom() {
			out = append(out, b)
		}
	}

	return out
}

func pairs(boxes []box) [][2]box {
	var all [][2]box
	for i, a := range boxes {
		for _, b := range boxes[i+1:] {
			all = append(all, [2]box{a, b})
		}
	}

	return all
}

func touching(boxes []box) [][2]box {
	var found [][2]box
	for _, pair := range pairs(boxes) {
		a, b := pair[0], pair[1]
		if a.x <= b.right() && b.x <= a.right() && a.y <= b.bottom() && b.y <= a.bottom() {
			found = append(found, pair)
		}
	}

	return found
}

func shrunk(b box, by int) box {
	return box{x: b.x + by, y: b.y + by, w: b.w - 2*by, h: b.h - 2*by}
}

func TestArrangeKeepsEveryControlInsideTheWindow(t *testing.T) {
	for _, dpi := range densities {
		p := arrange(sections, dpi)

		if out := misplaced(append(controlsOf(p), p.frames...), shrunk(box{w: p.width, h: p.height}, 4)); len(out) > 0 {
			t.Errorf("at %d dpi %+v are empty or at the edge of the %dx%d window", dpi, out, p.width, p.height)
		}
	}
}

func TestArrangeLetsNoTwoControlsTouch(t *testing.T) {
	for _, dpi := range densities {
		if found := touching(controlsOf(arrange(sections, dpi))); len(found) > 0 {
			t.Errorf("at %d dpi these touch: %+v", dpi, found)
		}
	}
}

func TestArrangeKeepsTheFieldsOfASectionInItsFrameUnderTheTitle(t *testing.T) {
	p := arrange(sections, 96)

	for _, s := range spots(p) {
		frame := p.frames[s.section]
		under := shrunk(box{x: frame.x, y: frame.y + 12, w: frame.w, h: frame.h - 12}, 4)

		if out := misplaced(s.row.boxes(), under); len(out) > 0 {
			t.Errorf("%s: %+v are not within %+v under its title", s.field.Label, out, frame)
		}
	}
}

func TestArrangeLeavesRoomBetweenTheFrames(t *testing.T) {
	var grown []box
	for _, f := range arrange(sections, 96).frames {
		grown = append(grown, box{x: f.x - 2, y: f.y - 2, w: f.w + 4, h: f.h + 4})
	}

	if found := touching(grown); len(found) > 0 {
		t.Errorf("these frames touch or overlap: %+v", found)
	}
}

func TestTheButtonsAndTheNoteSitBelowEverySection(t *testing.T) {
	p := arrange(sections, 96)

	for _, f := range p.frames {
		if f.bottom() >= p.note.y || f.bottom() >= p.save.y || f.bottom() >= p.cancel.y {
			t.Errorf("the frame %+v reaches down to the note %+v or the buttons", f, p.note)
		}
	}
}

func TestOnlyAPathGetsABrowseButtonAtTheEndOfItsRow(t *testing.T) {
	for _, s := range spots(arrange(sections, 96)) {
		wants := s.field.Kind == Program || s.field.Kind == Model
		browse, input := s.row.browse, s.row.input

		if (browse != box{}) != wants || wants && (browse.x <= input.right() || browse.y != input.y) {
			t.Errorf("%s: browse %+v beside input %+v, want one only for a path, right of it on its row", s.field.Label, browse, input)
		}
	}
}

func TestEveryInputOfAColumnStartsAtTheSameEdge(t *testing.T) {
	edges := map[int]int{}
	for _, s := range spots(arrange(sections, 96)) {
		column := sections[s.section].Column
		if _, seen := edges[column]; !seen {
			edges[column] = s.row.input.x
		}

		if s.row.input.x != edges[column] {
			t.Errorf("%s starts at %d, want %d like the rest of its column", s.field.Label, s.row.input.x, edges[column])
		}
	}
}

func TestArrangeGrowsWithTheScreenDensity(t *testing.T) {
	small, large := arrange(sections, 96), arrange(sections, 192)

	if d := large.width - 2*small.width; d < -4 || d > 4 {
		t.Errorf("at 192 dpi the window is %d wide, want about twice the %d at 96", large.width, small.width)
	}

	if d := large.height - 2*small.height; d < -4 || d > 4 {
		t.Errorf("at 192 dpi the window is %d high, want about twice the %d at 96", large.height, small.height)
	}
}

func TestTheWindowFitsAFullHDScreenScaledToOneAndAHalf(t *testing.T) {
	p := arrange(sections, 144)

	// 1920x1080 at 150% leaves about 1030 pixels above the taskbar, and the title bar and
	// the frame of the window take about 50 of them.
	if p.width > 1900 || p.height > 980 {
		t.Errorf("the window is %dx%d, want it to fit 1900x980", p.width, p.height)
	}
}

func TestTheButtonsCloseTheWindowAtItsBottomRight(t *testing.T) {
	p := arrange(sections, 96)

	if p.cancel.right() <= p.save.right() || p.save.bottom() != p.cancel.bottom() || p.height-p.cancel.bottom() > 20 || p.width-p.cancel.right() > 20 {
		t.Errorf("save %+v and cancel %+v in a %dx%d window, want them side by side at its bottom right", p.save, p.cancel, p.width, p.height)
	}
}
