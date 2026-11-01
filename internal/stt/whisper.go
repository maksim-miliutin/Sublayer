package stt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/maksim-miliutin/Sublayer/internal/post"
	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

type Config struct {
	URL      string
	Language string // a whisper language code, or auto to let whisper tell
}

type Whisper struct {
	config Config
	client *http.Client
}

type reply struct {
	Text *string `json:"text"`
}

func NewWhisper(c Config, client *http.Client) *Whisper {
	return &Whisper{config: c, client: client}
}

func (w *Whisper) Transcribe(ctx context.Context, speech []float32, rate int) (string, error) {
	r, err := w.request(wav.Encode(speech, rate))
	if err != nil {
		return "", err
	}

	answer, err := post.Send(ctx, w.client, r)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoAnswer, err)
	}

	return textOf(answer)
}

func (w *Whisper) request(file []byte) (post.Request, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)

	err := errors.Join(
		writeFile(form, file),
		form.WriteField("response_format", "json"),
		// whisper-server falls back to the language it was started with, English unless
		// told otherwise, so every request names its own.
		form.WriteField("language", w.config.Language),
		form.Close(),
	)
	if err != nil {
		return post.Request{}, err
	}

	return post.Request{URL: w.config.URL, ContentType: form.FormDataContentType(), Body: body.Bytes()}, nil
}

func writeFile(form *multipart.Writer, file []byte) error {
	part, err := form.CreateFormFile("file", "phrase.wav")
	if err != nil {
		return err
	}

	_, err = part.Write(file)

	return err
}

func textOf(answer post.Answer) (string, error) {
	var r reply
	if err := post.Decode(answer, &r); err != nil {
		return "", fmt.Errorf("%w: %w", ErrBadAnswer, err)
	}

	if r.Text == nil {
		return "", fmt.Errorf("%w: %s", ErrBadAnswer, answer.Status)
	}

	return oneLine(*r.Text), nil
}

// whisper-server ends every segment with a newline of its own, while the segments carry
// the spaces between words, and a segment cut short can end inside a word.
func oneLine(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(text, "\n", ""))
}
