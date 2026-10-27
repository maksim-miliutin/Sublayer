package capture

import (
	"testing"
	"time"
)

var start = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func TestFillerOwesNothingWhileSoundKeepsComing(t *testing.T) {
	f := newFiller(48000, start)

	for at := 10 * time.Millisecond; at <= time.Second; at += 10 * time.Millisecond {
		f.hear(start.Add(at))

		if owed := f.pay(start.Add(at + 5*time.Millisecond)); owed != 0 {
			t.Fatalf("at %v the filler owed %d frames while sound was coming", at, owed)
		}
	}
}

func TestFillerPaysForAQuietStretchOnceItOutgrowsTheSlack(t *testing.T) {
	f := newFiller(48000, start)

	if owed := f.pay(start.Add(quietSlack - time.Millisecond)); owed != 0 {
		t.Errorf("owed %d frames before the slack ran out", owed)
	}

	if owed := f.pay(start.Add(250 * time.Millisecond)); owed != 12000 {
		t.Errorf("owed %d frames for 250 ms of quiet at 48 kHz, want 12000", owed)
	}
}

func TestFillerNeverPaysForTheSameTimeTwice(t *testing.T) {
	const rate = 44100
	f := newFiller(rate, start)

	var paid int
	for at := 10 * time.Millisecond; at <= 10*time.Second; at += 10 * time.Millisecond {
		paid += f.pay(start.Add(at))
	}

	if paid > 10*rate {
		t.Errorf("paid %d frames for ten seconds, more than the %d they hold", paid, 10*rate)
	}

	if unpaid := 10*rate - paid; unpaid > int(quietSlack.Seconds()*rate) {
		t.Errorf("left %d frames of ten quiet seconds unpaid, more than the slack", unpaid)
	}
}

func TestFillerLosesNoTimeToRounding(t *testing.T) {
	const rate = 44100
	f := newFiller(rate, start)

	var paid int
	at := time.Duration(0)
	for ; at < time.Minute; at += 7 * time.Millisecond {
		paid += f.pay(start.Add(at))
	}

	end := at + 2*quietSlack
	paid += f.pay(start.Add(end))

	if want := int(end.Seconds() * rate); paid < want-1 || paid > want {
		t.Errorf("paid %d frames for %v of quiet, want %d", paid, end, want)
	}
}

func TestFillerCountsQuietFromTheLastSound(t *testing.T) {
	f := newFiller(48000, start)
	f.hear(start.Add(time.Second))

	if owed := f.pay(start.Add(time.Second + quietSlack/2)); owed != 0 {
		t.Errorf("owed %d frames half a slack after the last sound", owed)
	}

	if owed := f.pay(start.Add(time.Second + 200*time.Millisecond)); owed != 9600 {
		t.Errorf("owed %d frames for 200 ms after the last sound, want 9600", owed)
	}
}
