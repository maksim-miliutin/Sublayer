package capture

import (
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"testing"
)

var floatStereo48k = []byte{
	0xfe, 0xff,
	0x02, 0x00,
	0x80, 0xbb, 0x00, 0x00,
	0x00, 0xdc, 0x05, 0x00,
	0x08, 0x00,
	0x20, 0x00,
	0x16, 0x00,
	0x20, 0x00,
	0x03, 0x00, 0x00, 0x00,
	0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71,
}

var pcm16Mono44k = []byte{
	0x01, 0x00,
	0x01, 0x00,
	0x44, 0xac, 0x00, 0x00,
	0x88, 0x58, 0x01, 0x00,
	0x02, 0x00,
	0x10, 0x00,
	0x00, 0x00,
}

func patched(raw []byte, at int, value uint16) []byte {
	out := slices.Clone(raw)
	binary.LittleEndian.PutUint16(out[at:], value)

	return out
}

func TestParseFormatReadsTheUsualSharedModeMix(t *testing.T) {
	got, err := parseFormat(floatStereo48k)
	if err != nil {
		t.Fatal(err)
	}

	want := format{rate: 48000, channels: 2, block: 8, float: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseFormatReadsPlainSixteenBitPCM(t *testing.T) {
	got, err := parseFormat(pcm16Mono44k)
	if err != nil {
		t.Fatal(err)
	}

	want := format{rate: 44100, channels: 1, block: 2, float: false}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseFormatRefusesWhatItCannotDecode(t *testing.T) {
	cases := map[string][]byte{
		"cut short":              floatStereo48k[:17],
		"extensible cut short":   floatStereo48k[:30],
		"24-bit pcm":             patched(patched(pcm16Mono44k, 14, 24), 12, 3),
		"pcm subformat 32-bit":   patched(floatStereo48k, 24, 0x0001),
		"unknown subformat":      patched(floatStereo48k, 38, 0x0000),
		"frame size lies":        patched(floatStereo48k, 12, 6),
		"no channels":            patched(patched(floatStereo48k, 2, 0), 12, 0),
		"plain float 64-bit":     patched(patched(patched(pcm16Mono44k, 0, 3), 14, 64), 12, 8),
		"extensible cbSize lies": patched(floatStereo48k, 16, 0),
	}

	for name, raw := range cases {
		if _, err := parseFormat(raw); !errors.Is(err, ErrFormat) {
			t.Errorf("%s: got %v, want ErrFormat", name, err)
		}
	}
}

func floats(values ...float32) []byte {
	out := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(v))
	}

	return out
}

func TestMonoAveragesTheChannelsOfEachFrame(t *testing.T) {
	f := format{rate: 48000, channels: 2, block: 8, float: true}

	got := f.mono(floats(0.5, -0.5, 1, 0.5, -0.2, -0.4))

	want := []float32{0, 0.75, -0.3}
	if !slices.EqualFunc(got, want, func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-7 }) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMonoScalesSixteenBitSamplesIntoUnitRange(t *testing.T) {
	f := format{rate: 44100, channels: 1, block: 2, float: false}
	packet := []byte{0x00, 0x80, 0x00, 0x00, 0x00, 0x40, 0xff, 0x7f}

	got := f.mono(packet)

	want := []float32{-1, 0, 0.5, 32767.0 / 32768}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMonoAveragesSixteenBitStereo(t *testing.T) {
	f := format{rate: 44100, channels: 2, block: 4, float: false}
	packet := []byte{0x00, 0x40, 0x00, 0xc0, 0x00, 0x40, 0x00, 0x00}

	got := f.mono(packet)

	want := []float32{0, 0.25}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMonoIgnoresATrailingPartFrame(t *testing.T) {
	f := format{rate: 48000, channels: 2, block: 8, float: true}

	if got := f.mono(floats(0.5, 0.5, 0.25)); len(got) != 1 {
		t.Errorf("got %d frames from one and a half, want 1", len(got))
	}
}
