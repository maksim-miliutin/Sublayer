package vad

import (
	"slices"
	"time"
)

type Config struct {
	MinSpeech time.Duration
	EndPause  time.Duration
	MaxPhrase time.Duration
}

func Defaults() Config {
	return Config{MinSpeech: 300 * time.Millisecond, EndPause: 600 * time.Millisecond, MaxPhrase: 12 * time.Second}
}

type Phrase struct {
	Start, End time.Duration // from the start of the stream
	Samples    []float32
}

const (
	frameSpan = 20 * time.Millisecond
	preRoll   = 200 * time.Millisecond
	postRoll  = 200 * time.Millisecond
	floorSpan = 3 * time.Second

	onMargin  = 12.0
	offMargin = 6.0
)

type Segmenter struct {
	rate      int
	frame     int64
	minSpeech int
	endPause  int
	preRoll   int64
	postRoll  int64
	floor     *floor

	pending []float32
	kept    []float32
	base    int64
	at      int64

	open      bool
	start     int64
	voiced    int
	quiet     int
	lastVoice int64
}

func New(rate int, c Config) *Segmenter {
	frame := int64(rate) * int64(frameSpan) / int64(time.Second)
	frames := func(d time.Duration) int { return int((d + frameSpan - 1) / frameSpan) }
	samples := func(d time.Duration) int64 { return int64(rate) * int64(d) / int64(time.Second) }

	return &Segmenter{
		rate:      rate,
		frame:     frame,
		minSpeech: frames(c.MinSpeech),
		endPause:  frames(c.EndPause),
		preRoll:   samples(preRoll),
		postRoll:  samples(postRoll),
		floor:     newFloor(frames(floorSpan)),
	}
}

func (s *Segmenter) Feed(samples []float32) []Phrase {
	s.pending = append(s.pending, samples...)

	var out []Phrase
	for int64(len(s.pending)) >= s.frame {
		frame := s.pending[:s.frame]
		s.kept = append(s.kept, frame...)

		if p, done := s.step(frame); done {
			out = append(out, p)
		}

		s.pending = s.pending[s.frame:]
	}

	s.pending = slices.Clone(s.pending)
	s.trim()

	return out
}

func (s *Segmenter) Flush() []Phrase {
	if !s.open {
		return nil
	}

	s.open = false
	if s.voiced < s.minSpeech {
		return nil
	}

	return []Phrase{s.phrase(s.start, min(s.lastVoice+s.postRoll, s.at))}
}

func (s *Segmenter) step(frame []float32) (Phrase, bool) {
	level := loudness(frame)
	s.floor.add(level)
	floor := s.floor.level()

	end := s.at + s.frame
	defer func() { s.at = end }()

	if !s.open {
		if level < floor+onMargin {
			return Phrase{}, false
		}

		s.open = true
		s.start = max(0, s.at-s.preRoll)
		s.voiced, s.quiet = 0, 0
	}

	if level >= floor+offMargin {
		s.voiced++
		s.quiet = 0
		s.lastVoice = end
	} else {
		s.quiet++
	}

	if s.quiet < s.endPause {
		return Phrase{}, false
	}

	s.open = false
	if s.voiced < s.minSpeech {
		return Phrase{}, false
	}

	return s.phrase(s.start, min(s.lastVoice+s.postRoll, end)), true
}

func (s *Segmenter) phrase(from, to int64) Phrase {
	return Phrase{
		Start:   s.time(from),
		End:     s.time(to),
		Samples: slices.Clone(s.kept[from-s.base : to-s.base]),
	}
}

func (s *Segmenter) time(index int64) time.Duration {
	return time.Duration(index) * time.Second / time.Duration(s.rate)
}

func (s *Segmenter) trim() {
	keep := s.at - s.preRoll
	if s.open {
		keep = s.start
	}

	if drop := keep - s.base; drop > 0 {
		s.kept = append(s.kept[:0], s.kept[drop:]...)
		s.base = keep
	}
}
