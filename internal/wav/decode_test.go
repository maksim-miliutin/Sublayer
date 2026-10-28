package wav

import (
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"testing"
)

func chunk(id string, body []byte) []byte {
	out := append([]byte(id), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(body)))
	out = append(out, body...)
	if len(body)%2 == 1 {
		out = append(out, 0)
	}

	return out
}

func riff(chunks ...[]byte) []byte {
	body := []byte("WAVE")
	for _, c := range chunks {
		body = append(body, c...)
	}

	return chunk("RIFF", body)
}

func fmtChunk(tag, channels uint16, rate uint32, bits uint16) []byte {
	body := make([]byte, 16)
	binary.LittleEndian.PutUint16(body[0:], tag)
	binary.LittleEndian.PutUint16(body[2:], channels)
	binary.LittleEndian.PutUint32(body[4:], rate)
	binary.LittleEndian.PutUint32(body[8:], rate*uint32(channels)*uint32(bits)/8)
	binary.LittleEndian.PutUint16(body[12:], channels*bits/8)
	binary.LittleEndian.PutUint16(body[14:], bits)

	return chunk("fmt ", body)
}

func TestDecodeReadsBackWhatEncodeWrote(t *testing.T) {
	in := []float32{0, 0.5, -0.5, 0.999, -1, 0.123}

	got, rate, err := Decode(Encode(in, 16000))
	if err != nil {
		t.Fatal(err)
	}

	if rate != 16000 || len(got) != len(in) {
		t.Fatalf("got %d samples at %d Hz, want %d at 16000", len(got), rate, len(in))
	}

	for i := range in {
		if math.Abs(float64(got[i]-in[i])) > 0.5/32767 {
			t.Errorf("sample %d came back as %v, want %v", i, got[i], in[i])
		}
	}
}

func TestDecodeSkipsChunksItDoesNotNeed(t *testing.T) {
	file := riff(fmtChunk(1, 1, 16000, 16), chunk("LIST", []byte("odd")), chunk("data", []byte{0x00, 0x40, 0x00, 0xc0}))

	got, _, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if want := []float32{16384.0 / 32767, -16384.0 / 32767}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v past an odd-sized chunk", got, want)
	}
}

func TestDecodeTakesWhatThereIsOfACutOffData(t *testing.T) {
	file := riff(fmtChunk(1, 1, 16000, 16), chunk("data", []byte{0x00, 0x40, 0x00, 0xc0}))
	binary.LittleEndian.PutUint32(file[len(file)-8:], 1000)

	got, _, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Errorf("got %d samples, want the 2 actually present", len(got))
	}
}

func TestDecodeKeepsTheLoudestForeignSampleInRange(t *testing.T) {
	file := riff(fmtChunk(1, 1, 16000, 16), chunk("data", []byte{0x00, 0x80, 0xff, 0x7f}))

	got, _, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(got, []float32{-1, 1}) {
		t.Errorf("got %v, want the extremes at -1 and 1", got)
	}
}

func TestDecodeRefusesWhatItCannotRead(t *testing.T) {
	data := chunk("data", []byte{0, 0})
	cases := map[string][]byte{
		"empty":      nil,
		"not riff":   append([]byte("RIFX"), riff(fmtChunk(1, 1, 16000, 16), data)[4:]...),
		"stereo":     riff(fmtChunk(1, 2, 16000, 16), data),
		"8-bit":      riff(fmtChunk(1, 1, 16000, 8), data),
		"float":      riff(fmtChunk(3, 1, 16000, 32), data),
		"no format":  riff(data),
		"no data":    riff(fmtChunk(1, 1, 16000, 16)),
		"short fmt":  riff(chunk("fmt ", []byte{1, 0, 1, 0}), data),
		"no rate":    riff(fmtChunk(1, 1, 0, 16), data),
		"chunk lies": riff(chunk("fmt ", make([]byte, 16)))[:20],
	}

	for name, file := range cases {
		if _, _, err := Decode(file); !errors.Is(err, ErrFormat) {
			t.Errorf("%s: got %v, want ErrFormat", name, err)
		}
	}
}
