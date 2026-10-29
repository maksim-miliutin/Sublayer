package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

const answerLimit = 1 << 20 // bytes; the text of a phrase is a few hundred

type Config struct {
	URL      string
	Language string // a whisper language code, or auto to let whisper tell
}

type Whisper struct {
	config Config
	client *http.Client
}

type form struct {
	body        []byte
	contentType string
}

type answer struct {
	Text  *string `json:"text"`
	Error string  `json:"error"`
}

func NewWhisper(c Config, client *http.Client) *Whisper {
	return &Whisper{config: c, client: client}
}

func (w *Whisper) Transcribe(ctx context.Context, speech []float32, rate int) (string, error) {
	f, err := newForm(wav.Encode(speech, rate), w.config.Language)
	if err != nil {
		return "", err
	}

	// A connection the server closed while idle fails a request it never saw, and one
	// more try on a fresh connection settles that; a slow server is not asked twice.
	text, err := w.ask(ctx, f)
	if !errors.Is(err, ErrNoAnswer) || timedOut(err) || ctx.Err() != nil {
		return text, err
	}

	return w.ask(ctx, f)
}

func newForm(file []byte, language string) (form, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	err := errors.Join(
		writeFile(w, file),
		w.WriteField("response_format", "json"),
		// whisper-server falls back to the language it was started with, English unless
		// told otherwise, so every request names its own.
		w.WriteField("language", language),
		w.Close(),
	)
	if err != nil {
		return form{}, err
	}

	return form{body: body.Bytes(), contentType: w.FormDataContentType()}, nil
}

func writeFile(w *multipart.Writer, file []byte) error {
	part, err := w.CreateFormFile("file", "phrase.wav")
	if err != nil {
		return err
	}

	_, err = part.Write(file)

	return err
}

func (w *Whisper) ask(ctx context.Context, f form) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.URL, bytes.NewReader(f.body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", f.contentType)

	response, err := w.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoAnswer, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, answerLimit))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoAnswer, err)
	}

	return textOf(response, body)
}

func textOf(response *http.Response, body []byte) (string, error) {
	var a answer
	if err := json.Unmarshal(body, &a); err != nil {
		return "", fmt.Errorf("%w: %s: %s", ErrBadAnswer, response.Status, strings.Join(strings.Fields(string(body)), " "))
	}

	if a.Error != "" {
		return "", fmt.Errorf("%w: %s: %s", ErrBadAnswer, response.Status, a.Error)
	}

	if response.StatusCode != http.StatusOK || a.Text == nil {
		return "", fmt.Errorf("%w: %s", ErrBadAnswer, response.Status)
	}

	return oneLine(*a.Text), nil
}

// whisper-server ends every segment with a newline of its own, while the segments carry
// the spaces between words, and a segment cut short can end inside a word.
func oneLine(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(text, "\n", ""))
}

func timedOut(err error) bool {
	var timeout interface{ Timeout() bool }

	return errors.As(err, &timeout) && timeout.Timeout()
}
