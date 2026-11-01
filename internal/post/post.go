package post

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const answerLimit = 1 << 20 // bytes; a phrase or its translation runs to a few hundred

type Request struct {
	URL         string
	ContentType string
	Body        []byte
}

type Answer struct {
	Status string
	Code   int
	Body   []byte
}

func Send(ctx context.Context, client *http.Client, r Request) (Answer, error) {
	// A connection the server closed while idle fails a request it never saw, and one
	// more try on a fresh connection settles that; a slow server is not asked twice.
	answer, err := once(ctx, client, r)
	if err == nil || timedOut(err) || ctx.Err() != nil {
		return answer, err
	}

	return once(ctx, client, r)
}

func once(ctx context.Context, client *http.Client, r Request) (Answer, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(r.Body))
	if err != nil {
		return Answer{}, err
	}
	request.Header.Set("Content-Type", r.ContentType)

	response, err := client.Do(request)
	if err != nil {
		return Answer{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, answerLimit))
	if err != nil {
		return Answer{}, err
	}

	return Answer{Status: response.Status, Code: response.StatusCode, Body: body}, nil
}

func Decode(answer Answer, v any) error {
	var fault struct {
		Error string `json:"error"`
	}

	if json.Unmarshal(answer.Body, &fault) != nil || json.Unmarshal(answer.Body, v) != nil {
		return fmt.Errorf("%s: %s", answer.Status, strings.Join(strings.Fields(string(answer.Body)), " "))
	}

	if fault.Error != "" {
		return fmt.Errorf("%s: %s", answer.Status, fault.Error)
	}

	if answer.Code != http.StatusOK {
		return errors.New(answer.Status)
	}

	return nil
}

func timedOut(err error) bool {
	var timeout interface{ Timeout() bool }

	return errors.As(err, &timeout) && timeout.Timeout()
}
