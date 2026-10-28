package ui

import (
	"fmt"
	"testing"
)

var m96 = scaledMetrics(96)

func numbered(n int) []Pair {
	out := make([]Pair, n)
	for i := range out {
		out[i] = Pair{Original: fmt.Sprintf("original %d", i), Translation: fmt.Sprintf("translation %d", i)}
	}

	return out
}

func even(n, original, translation int) []heights {
	out := make([]heights, n)
	for i := range out {
		out[i] = heights{original: original, translation: translation}
	}

	return out
}

func TestArrangeStacksTheNewestPairAtTheBottom(t *testing.T) {
	s := arrange(Frame{Pairs: numbered(3)}, even(3, 18, 26), 400, 1000, m96)

	if len(s.plates) != 3 {
		t.Fatalf("got %d plates, want 3", len(s.plates))
	}

	for i := 1; i < len(s.plates); i++ {
		if s.plates[i].area.y < s.plates[i-1].area.bottom()+m96.gap {
			t.Errorf("plate %d starts at %d, inside or too close to plate %d ending at %d", i, s.plates[i].area.y, i-1, s.plates[i-1].area.bottom())
		}
	}

	if last := s.plates[2].area.bottom(); last != s.height {
		t.Errorf("the newest plate ends at %d, the scene at %d", last, s.height)
	}

	if s.blocks[len(s.blocks)-1].text != "translation 2" {
		t.Errorf("the last block says %q, want the newest translation", s.blocks[len(s.blocks)-1].text)
	}
}

func TestArrangeWrapsEachPairInItsPlate(t *testing.T) {
	s := arrange(Frame{Pairs: numbered(1)}, even(1, 18, 26), 400, 1000, m96)

	p := s.plates[0].area
	if want := m96.pad + 18 + m96.line + 26 + m96.pad; p.h != want {
		t.Errorf("plate is %d high, want %d", p.h, want)
	}

	if p.x != 0 || p.w != 400 {
		t.Errorf("plate spans %d+%d, want the whole width 400", p.x, p.w)
	}

	original, translation := s.blocks[0].area, s.blocks[1].area
	if original.x != m96.pad || original.w != 400-2*m96.pad || original.y != p.y+m96.pad || original.h != 18 {
		t.Errorf("original sits at %+v inside plate %+v", original, p)
	}

	if translation.y != original.bottom()+m96.line || translation.h != 26 || translation.bottom()+m96.pad != p.bottom() {
		t.Errorf("translation sits at %+v under original %+v in plate %+v", translation, original, p)
	}

	if !s.blocks[1].translation || s.blocks[0].translation {
		t.Error("the blocks do not say which font they want")
	}
}

func TestArrangeFadesTheTextOfOlderPairsDownToAFloor(t *testing.T) {
	s := arrange(Frame{Pairs: numbered(6)}, even(6, 18, 26), 400, 2000, m96)

	for i := 2; i < len(s.blocks); i += 2 {
		if s.blocks[i].alpha < s.blocks[i-2].alpha {
			t.Errorf("block %q is fainter than the older %q", s.blocks[i].text, s.blocks[i-2].text)
		}
	}

	if s.blocks[0].alpha >= s.blocks[len(s.blocks)-1].alpha {
		t.Error("the oldest text is as bright as the newest")
	}

	for _, b := range s.blocks {
		if b.alpha < fadeFloor || b.alpha > 1 {
			t.Errorf("block %q has alpha %v outside [%v, 1]", b.text, b.alpha, fadeFloor)
		}
	}

	if last := s.blocks[len(s.blocks)-1].alpha; last != 1 {
		t.Errorf("the newest translation has alpha %v, want 1", last)
	}
}

// A plate that faded with its text let a light window shine through and left white
// text unreadable on it, so plates stay equally dark however old their pair is.
func TestArrangeKeepsEveryPlateEquallyDark(t *testing.T) {
	s := arrange(Frame{Pairs: numbered(6)}, even(6, 18, 26), 400, 2000, m96)

	for i, p := range s.plates {
		if p.alpha != plateAlpha {
			t.Errorf("plate %d has alpha %v, want %v", i, p.alpha, plateAlpha)
		}
	}
}

func TestArrangeDropsTheOldestPairsThatDoNotFit(t *testing.T) {
	plate := m96.pad + 18 + m96.line + 26 + m96.pad
	room := m96.dot + 2*(m96.gap+plate)

	s := arrange(Frame{Pairs: numbered(5)}, even(5, 18, 26), 400, room, m96)

	if len(s.plates) != 2 {
		t.Fatalf("got %d plates in room for two, want 2", len(s.plates))
	}

	if s.height > room {
		t.Errorf("scene is %d high, more than the %d it was given", s.height, room)
	}

	if s.blocks[0].text != "original 3" {
		t.Errorf("the first block says %q, want the second newest pair", s.blocks[0].text)
	}
}

func TestArrangeKeepsTheNewestPairEvenWhenItAloneIsTooTall(t *testing.T) {
	s := arrange(Frame{Pairs: numbered(3)}, even(3, 300, 300), 400, 100, m96)

	if len(s.plates) != 1 || s.blocks[1].text != "translation 2" {
		t.Errorf("got %d plates ending with %q, want only the newest pair", len(s.plates), s.blocks[len(s.blocks)-1].text)
	}
}

func TestArrangeShowsOnlyTheStateDotWithNothingToSay(t *testing.T) {
	s := arrange(Frame{State: Paused}, nil, 400, 1000, m96)

	if len(s.plates) != 0 || len(s.blocks) != 0 {
		t.Errorf("got %d plates and %d blocks for no pairs", len(s.plates), len(s.blocks))
	}

	if s.height != s.dot.bottom() || s.dot.h == 0 {
		t.Errorf("scene is %d high around a dot %+v, want just the dot", s.height, s.dot)
	}

	if s.dot.x+s.dot.w > s.width {
		t.Errorf("dot %+v sticks out of a scene %d wide", s.dot, s.width)
	}
}

func TestArrangeLeavesNoRoomForAMissingOriginal(t *testing.T) {
	f := Frame{Pairs: []Pair{{Translation: "only this"}}}

	s := arrange(f, []heights{{original: 0, translation: 26}}, 400, 1000, m96)

	if len(s.blocks) != 1 || s.blocks[0].text != "only this" {
		t.Fatalf("got blocks %+v, want the translation alone", s.blocks)
	}

	if want := m96.pad + 26 + m96.pad; s.plates[0].area.h != want {
		t.Errorf("plate is %d high, want %d with no line gap", s.plates[0].area.h, want)
	}
}

func TestEveryStateHasItsOwnColour(t *testing.T) {
	seen := map[rgb]State{}
	for _, s := range []State{Listening, Recognizing, Paused, Failed} {
		c := stateColor(s)
		if other, taken := seen[c]; taken {
			t.Errorf("states %d and %d share colour %v", other, s, c)
		}

		seen[c] = s
	}

	if stateColor(State(99)) != stateColor(Failed) {
		t.Error("an unknown state is not shown as a failure")
	}
}

func TestPlaceSitsAtTheBottomLeftOfTheWorkArea(t *testing.T) {
	cases := []struct {
		name string
		work rect
	}{
		{"taskbar at the bottom", rect{x: 0, y: 0, w: 1920, h: 1040}},
		{"taskbar on the left", rect{x: 62, y: 0, w: 1858, h: 1080}},
		{"taskbar at the top", rect{x: 0, y: 48, w: 2560, h: 1392}},
	}

	for _, c := range cases {
		width := widthFor(c.work)
		got := place(c.work, width, 300, m96)

		if got.x != c.work.x+m96.margin {
			t.Errorf("%s: left edge at %d, want %d", c.name, got.x, c.work.x+m96.margin)
		}

		if got.bottom() != c.work.bottom()-m96.margin {
			t.Errorf("%s: bottom at %d, want %d", c.name, got.bottom(), c.work.bottom()-m96.margin)
		}

		if want := c.work.w * 3 / 10; got.w != want || got.h != 300 {
			t.Errorf("%s: size %dx%d, want %dx300", c.name, got.w, got.h, want)
		}
	}
}

func TestMetricsGrowWithTheScreenDensity(t *testing.T) {
	at96, at144 := scaledMetrics(96), scaledMetrics(144)

	if at144.pad != at96.pad*3/2 || at144.dot != at96.dot*3/2 || at144.margin != at96.margin*3/2 {
		t.Errorf("at 150%% got %+v from %+v", at144, at96)
	}
}
