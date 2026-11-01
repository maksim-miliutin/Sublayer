package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/maksim-miliutin/Sublayer/internal/post"
)

type Config struct {
	URL    string
	Source string // a language code, or auto to let LibreTranslate tell
	Target string
}

type Libre struct {
	config Config
	client *http.Client
}

type question struct {
	Text   string `json:"q"`
	Source string `json:"source"`
	Target string `json:"target"`
	Format string `json:"format"`
}

type reply struct {
	Translation *string `json:"translatedText"`
}

func NewLibre(c Config, client *http.Client) *Libre {
	return &Libre{config: c, client: client}
}

func (l *Libre) Translate(ctx context.Context, text string) (string, error) {
	body, err := json.Marshal(question{Text: text, Source: l.config.Source, Target: l.config.Target, Format: "text"})
	if err != nil {
		return "", err
	}

	answer, err := post.Send(ctx, l.client, post.Request{URL: l.config.URL, ContentType: "application/json", Body: body})
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoAnswer, err)
	}

	var r reply
	if err := post.Decode(answer, &r); err != nil {
		return "", fmt.Errorf("%w: %w", ErrBadAnswer, err)
	}

	if r.Translation == nil {
		return "", fmt.Errorf("%w: %s", ErrBadAnswer, answer.Status)
	}

	return strings.TrimSpace(*r.Translation), nil
}
