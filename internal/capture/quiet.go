package capture

import "time"

const quietSlack = 100 * time.Millisecond

type filler struct {
	rate int
	last time.Time
}

func newFiller(rate int, now time.Time) *filler {
	return &filler{rate: rate, last: now}
}

func (f *filler) hear(now time.Time) {
	f.last = now
}

func (f *filler) pay(now time.Time) int {
	gap := now.Sub(f.last)
	if gap < quietSlack {
		return 0
	}

	frames := int(gap * time.Duration(f.rate) / time.Second)
	f.last = f.last.Add(time.Duration(frames) * time.Second / time.Duration(f.rate))

	return frames
}
