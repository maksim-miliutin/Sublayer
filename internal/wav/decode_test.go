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

type waveFormat struct {
	tag      uint16
	channels uint16
	rate     uint32
	bits     uint16
}

var plainPCM = waveFormat{tag: 1, channels: 1, rate: 16000, bits: 16}

func (f waveFormat) chunk() []byte {
	body := make([]byte, 16)
	binary.LittleEndian.PutUint16(body[0:], f.tag)
	binary.LittleEndian.PutUint16(body[2:], f.channels)
	binary.LittleEndian.PutUint32(body[4:], f.rate)
	binary.LittleEndian.PutUint32(body[8:], f.rate*uint32(f.channels)*uint32(f.bits)/8)
	binary.LittleEndian.PutUint16(body[12:], f.channels*f.bits/8)
	binary.LittleEndian.PutUint16(body[14:], f.bits)

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

func TestDecodeReadsBackAnEmptyRecording(t *testing.T) {
	got, rate, err := Decode(Encode(nil, 16000))
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 0 || rate != 16000 {
		t.Errorf("got %d samples at %d Hz, want none at 16000", len(got), rate)
	}
}

func TestDecodeSkipsChunksItDoesNotNeed(t *testing.T) {
	file := riff(plainPCM.chunk(), chunk("LIST", []byte("odd")), chunk("data", []byte{0x00, 0x40, 0x00, 0xc0}))

	got, _, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if want := []float32{16384.0 / 32767, -16384.0 / 32767}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v past an odd-sized chunk", got, want)
	}
}

func TestDecodeTakesWhatThereIsOfACutOffData(t *testing.T) {
	file := riff(plainPCM.chunk(), chunk("data", []byte{0x00, 0x40, 0x00, 0xc0}))
	binary.LittleEndian.PutUint32(file[len(file)-8:], 1000)

	got, _, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Errorf("got %d samples, want the 2 actually present", len(got))
	}
}

// The bug this guards: a streamed file writes its data size as all ones, which turned
// negative in a 32-bit int and the slice of the chunk panicked.
func TestDecodeTakesTheRestOfTheFileForAnEndlessDataSize(t *testing.T) {
	file := riff(plainPCM.chunk(), chunk("data", []byte{0x00, 0x40, 0x00, 0xc0}))
	binary.LittleEndian.PutUint32(file[len(file)-8:], 0xffffffff)

	got, _, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Errorf("got %d samples, want the 2 that follow an endless data size", len(got))
	}
}

func TestDecodeKeepsTheLoudestForeignSampleInRange(t *testing.T) {
	file := riff(plainPCM.chunk(), chunk("data", []byte{0x00, 0x80, 0xff, 0x7f}))

	got, _, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(got, []float32{-1, 1}) {
		t.Errorf("got %v, want the extremes at -1 and 1", got)
	}
}

func TestDecodeRefusesWhatItCannotRead(t *testing.T) {
	pcm := chunk("data", []byte{0, 0})
	cases := map[string][]byte{
		"empty":      nil,
		"not riff":   append([]byte("RIFX"), riff(plainPCM.chunk(), pcm)[4:]...),
		"stereo":     riff(waveFormat{tag: 1, channels: 2, rate: 16000, bits: 16}.chunk(), pcm),
		"8-bit":      riff(waveFormat{tag: 1, channels: 1, rate: 16000, bits: 8}.chunk(), pcm),
		"float":      riff(waveFormat{tag: 3, channels: 1, rate: 16000, bits: 32}.chunk(), pcm),
		"no format":  riff(pcm),
		"no data":    riff(plainPCM.chunk()),
		"short fmt":  riff(chunk("fmt ", []byte{1, 0, 1, 0}), pcm),
		"no rate":    riff(waveFormat{tag: 1, channels: 1, rate: 0, bits: 16}.chunk(), pcm),
		"chunk lies": riff(chunk("fmt ", make([]byte, 16)))[:20],
	}

	for name, file := range cases {
		if _, _, err := Decode(file); !errors.Is(err, ErrFormat) {
			t.Errorf("%s: got %v, want ErrFormat", name, err)
		}
	}
}
