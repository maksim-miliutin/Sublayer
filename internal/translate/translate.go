package translate

import (
	"context"
	"errors"
)

var (
	ErrNoAnswer  = errors.New("translate: no answer from LibreTranslate")
	ErrBadAnswer = errors.New("translate: LibreTranslate answered without a translation")
)

type Translator interface {
	Translate(ctx context.Context, text string) (string, error)
}
