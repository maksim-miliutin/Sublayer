package wav

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrFormat = errors.New("wav: not a 16-bit mono pcm file")

func Decode(file []byte) ([]float32, int, error) {
	if len(file) < 12 || string(file[0:4]) != "RIFF" || string(file[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("%w: no riff wave header", ErrFormat)
	}

	var format, pcm []byte
	for at := 12; at+8 <= len(file); {
		id := string(file[at : at+4])
		size := int(binary.LittleEndian.Uint32(file[at+4:]))
		body := file[at+8 : min(at+8+size, len(file))]

		switch id {
		case "fmt ":
			format = body
		case "data":
			pcm = body
		}

		at += 8 + size + size%2
	}

	rate, err := readFormat(format)
	if err != nil {
		return nil, 0, err
	}

	if pcm == nil {
		return nil, 0, fmt.Errorf("%w: no data chunk", ErrFormat)
	}

	samples := make([]float32, len(pcm)/bytesPerSample)
	for i := range samples {
		samples[i] = max(-1, float32(int16(binary.LittleEndian.Uint16(pcm[bytesPerSample*i:])))/fullScale)
	}

	return samples, rate, nil
}

func readFormat(format []byte) (int, error) {
	if len(format) < fmtChunkLen {
		return 0, fmt.Errorf("%w: no fmt chunk", ErrFormat)
	}

	tag := binary.LittleEndian.Uint16(format[0:])
	count := binary.LittleEndian.Uint16(format[2:])
	rate := int(binary.LittleEndian.Uint32(format[4:]))
	bits := binary.LittleEndian.Uint16(format[14:])

	if tag != formatPCM || count != channels || bits != 8*bytesPerSample || rate < 1 {
		return 0, fmt.Errorf("%w: tag %d, %d channels, %d bits, %d Hz", ErrFormat, tag, count, bits, rate)
	}

	return rate, nil
}
