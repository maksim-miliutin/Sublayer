package capture

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

var ErrFormat = errors.New("capture: the device mixes sound in a format this program cannot read")

const (
	tagPCM        = 0x0001
	tagFloat      = 0x0003
	tagExtensible = 0xfffe

	plainLen      = 18
	extensibleLen = 40
)

// An extensible format names its encoding by a GUID that is the plain format tag
// followed by these fourteen bytes, the same for every tag.
var subformatTail = []byte{0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71}

type format struct {
	rate     int
	channels int
	block    int
	float    bool
}

func parseFormat(raw []byte) (format, error) {
	if len(raw) < plainLen {
		return format{}, fmt.Errorf("%w: %d bytes of format", ErrFormat, len(raw))
	}

	tag := binary.LittleEndian.Uint16(raw[0:])
	if tag == tagExtensible {
		extra := int(binary.LittleEndian.Uint16(raw[16:]))
		if plainLen+extra < extensibleLen || len(raw) < extensibleLen {
			return format{}, fmt.Errorf("%w: extensible format of %d bytes", ErrFormat, len(raw))
		}

		if !bytes.Equal(raw[26:extensibleLen], subformatTail) {
			return format{}, fmt.Errorf("%w: unknown subformat % x", ErrFormat, raw[24:extensibleLen])
		}

		tag = binary.LittleEndian.Uint16(raw[24:])
	}

	f := format{
		channels: int(binary.LittleEndian.Uint16(raw[2:])),
		rate:     int(binary.LittleEndian.Uint32(raw[4:])),
		block:    int(binary.LittleEndian.Uint16(raw[12:])),
		float:    tag == tagFloat,
	}
	bits := int(binary.LittleEndian.Uint16(raw[14:]))

	known := (tag == tagFloat && bits == 32) || (tag == tagPCM && bits == 16)
	if !known {
		return format{}, fmt.Errorf("%w: tag %#04x with %d bits", ErrFormat, tag, bits)
	}

	if f.channels < 1 || f.rate < 1 || f.block != f.channels*bits/8 {
		return format{}, fmt.Errorf("%w: %d channels at %d Hz in %d-byte frames", ErrFormat, f.channels, f.rate, f.block)
	}

	return f, nil
}

func (f format) mono(packet []byte) []float32 {
	out := make([]float32, len(packet)/f.block)
	width := f.block / f.channels

	for i := range out {
		frame := packet[i*f.block:]

		var sum float32
		for c := range f.channels {
			sum += f.sample(frame[c*width:])
		}

		out[i] = sum / float32(f.channels)
	}

	return out
}

func (f format) sample(at []byte) float32 {
	if f.float {
		return math.Float32frombits(binary.LittleEndian.Uint32(at))
	}

	return float32(int16(binary.LittleEndian.Uint16(at))) / 32768
}
