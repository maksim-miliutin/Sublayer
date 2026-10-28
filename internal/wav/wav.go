package wav

import (
	"encoding/binary"
	"math"
)

const (
	headerLen      = 44
	fmtChunkLen    = 16
	formatPCM      = 1
	channels       = 1
	bytesPerSample = 2
	fullScale      = math.MaxInt16
)

func Encode(samples []float32, rate int) []byte {
	pcmLen := bytesPerSample * len(samples)
	out := make([]byte, headerLen+pcmLen)

	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(headerLen-8+pcmLen))
	copy(out[8:], "WAVE")

	copy(out[12:], "fmt ")
	binary.LittleEndian.PutUint32(out[16:], fmtChunkLen)
	binary.LittleEndian.PutUint16(out[20:], formatPCM)
	binary.LittleEndian.PutUint16(out[22:], channels)
	binary.LittleEndian.PutUint32(out[24:], uint32(rate))
	binary.LittleEndian.PutUint32(out[28:], uint32(rate*channels*bytesPerSample))
	binary.LittleEndian.PutUint16(out[32:], channels*bytesPerSample)
	binary.LittleEndian.PutUint16(out[34:], 8*bytesPerSample)

	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(pcmLen))

	for i, s := range samples {
		binary.LittleEndian.PutUint16(out[headerLen+bytesPerSample*i:], uint16(pcm16(s)))
	}

	return out
}

func pcm16(s float32) int16 {
	if math.IsNaN(float64(s)) {
		return 0
	}

	if s >= 1 {
		return fullScale
	}

	if s <= -1 {
		return -fullScale
	}

	return int16(math.Round(float64(s) * fullScale))
}
