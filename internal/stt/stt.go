package stt

import (
	"context"
	"errors"
)

var (
	ErrNoAnswer  = errors.New("stt: no answer from whisper-server")
	ErrBadAnswer = errors.New("stt: whisper-server answered without the text")
)

type Transcriber interface {
	Transcribe(ctx context.Context, speech []float32, rate int) (string, error)
}
