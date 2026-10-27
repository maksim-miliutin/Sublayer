package wav

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestEncodeWritesTheHeaderTheSpecDescribes(t *testing.T) {
	want := []byte{
		'R', 'I', 'F', 'F', 42, 0, 0, 0, 'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ', 16, 0, 0, 0,
		1, 0, 1, 0,
		0x80, 0x3e, 0, 0,
		0x00, 0x7d, 0, 0,
		2, 0, 16, 0,
		'd', 'a', 't', 'a', 6, 0, 0, 0,
		0x00, 0x00,
		0xff, 0x7f,
		0x01, 0x80,
	}

	got := Encode([]float32{0, 1, -1}, 16000)
	if !bytes.Equal(got, want) {
		t.Errorf("got\n% x\nwant\n% x", got, want)
	}
}

func TestEncodeOfNothingIsAValidEmptyFile(t *testing.T) {
	got := Encode(nil, 16000)

	if len(got) != 44 {
		t.Fatalf("got %d bytes, want a bare 44-byte header", len(got))
	}

	if size := binary.LittleEndian.Uint32(got[4:]); size != 36 {
		t.Errorf("riff size %d, want 36", size)
	}

	if size := binary.LittleEndian.Uint32(got[40:]); size != 0 {
		t.Errorf("data size %d, want 0", size)
	}
}

func TestEncodeClipsLoudSamplesInsteadOfWrapping(t *testing.T) {
	cases := []struct {
		in   float32
		want int16
	}{
		{1.5, 32767},
		{-2, -32767},
		{0.5, 16384},
		{-0.25, -8192},
		{float32(math.NaN()), 0},
	}

	for _, c := range cases {
		got := Encode([]float32{c.in}, 16000)
		if sample := int16(binary.LittleEndian.Uint16(got[44:])); sample != c.want {
			t.Errorf("%v became %d, want %d", c.in, sample, c.want)
		}
	}
}
