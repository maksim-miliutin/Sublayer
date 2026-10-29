package stt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

const inference = "http://whisper.test/inference"

var errDropped = errors.New("connection reset by peer")

type server struct {
	answer   func(attempt int, r *http.Request) (*http.Response, error)
	requests []*http.Request
	bodies   [][]byte
}

func (s *server) RoundTrip(r *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}

	s.requests = append(s.requests, r)
	s.bodies = append(s.bodies, body)

	return s.answer(len(s.requests)-1, r)
}

func reply(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func answering(status int, body string) func(int, *http.Request) (*http.Response, error) {
	return func(int, *http.Request) (*http.Response, error) {
		return reply(status, body), nil
	}
}

func whisperFor(s *server, language string) *Whisper {
	return NewWhisper(Config{URL: inference, Language: language}, &http.Client{Transport: s})
}

func formFields(t *testing.T, r *http.Request, body []byte) map[string][]byte {
	t.Helper()

	kind, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "multipart/form-data" {
		t.Fatalf("the request is %q, want a multipart form", r.Header.Get("Content-Type"))
	}

	fields := map[string][]byte{}
	parts := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := parts.NextPart()
		if errors.Is(err, io.EOF) {
			return fields
		}
		if err != nil {
			t.Fatal(err)
		}

		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}

		fields[part.FormName()] = content
	}
}

func TestWhisperSendsThePhraseAsAWavFileAndAsksForJSONInItsLanguage(t *testing.T) {
	s := &server{answer: answering(200, `{"text":" Oui."}`)}
	speech := []float32{0, 0.25, -0.5, 0.75, -1}

	if _, err := whisperFor(s, "fr").Transcribe(context.Background(), speech, 22050); err != nil {
		t.Fatal(err)
	}

	r := s.requests[0]
	if r.Method != http.MethodPost || r.URL.String() != inference {
		t.Fatalf("got %s %s, want POST %s", r.Method, r.URL, inference)
	}

	fields := formFields(t, r, s.bodies[0])
	for name, want := range map[string]string{"response_format": "json", "language": "fr"} {
		if got := string(fields[name]); got != want {
			t.Errorf("field %s is %q, want %q", name, got, want)
		}
	}

	samples, rate, err := wav.Decode(fields["file"])
	if err != nil {
		t.Fatalf("the file field is no wav: %v", err)
	}

	if rate != 22050 || len(samples) != len(speech) {
		t.Fatalf("the file holds %d samples at %d Hz, want %d at 22050", len(samples), rate, len(speech))
	}

	for i := range speech {
		if math.Abs(float64(samples[i]-speech[i])) > 1.0/32767 {
			t.Errorf("sample %d went as %v, want %v", i, samples[i], speech[i])
		}
	}
}

func TestWhisperJoinsTheSegmentsOfItsAnswerIntoOneLine(t *testing.T) {
	cases := map[string]string{
		`{"text":" Bonjour à tous.\n Comment ça va ?\n"}`: "Bonjour à tous. Comment ça va ?",
		`{"text":" Nous sommes aujourd'hu\ni réunis.\n"}`: "Nous sommes aujourd'hui réunis.",
		`{"text":"\n"}`: "",
	}

	for answer, want := range cases {
		got, err := whisperFor(&server{answer: answering(200, answer)}, "fr").Transcribe(context.Background(), []float32{0}, 16000)
		if err != nil {
			t.Fatalf("%s: %v", answer, err)
		}

		if got != want {
			t.Errorf("%s gave %q, want %q", answer, got, want)
		}
	}
}

func TestWhisperPassesOnWhyTheServerGaveNoText(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{400, "Invalid request", "400 Bad Request: Invalid request"},
		{404, "File Not Found (/inferenc)", "404 Not Found: File Not Found (/inferenc)"},
		{500, `{"error":"failed to process audio"}`, "500 Internal Server Error: failed to process audio"},
		{200, `{"error":"no 'file' field in the request"}`, "200 OK: no 'file' field in the request"},
		{500, `{"text":" Oui."}`, "500 Internal Server Error"},
		{200, `{"txt":" Oui."}`, "200 OK"},
		{200, " Oui.\n", "200 OK: Oui."},
	}

	for _, c := range cases {
		s := &server{answer: answering(c.status, c.body)}

		_, err := whisperFor(s, "fr").Transcribe(context.Background(), []float32{0}, 16000)
		if !errors.Is(err, ErrBadAnswer) || !strings.HasSuffix(err.Error(), ": "+c.want) {
			t.Errorf("%d %q gave %v, want ErrBadAnswer ending in %q", c.status, c.body, err, c.want)
		}

		if len(s.requests) != 1 {
			t.Errorf("%d %q was asked %d times, want once", c.status, c.body, len(s.requests))
		}
	}
}

func TestWhisperAsksOnceMoreWhenTheFirstAttemptGotNoAnswer(t *testing.T) {
	s := &server{answer: func(attempt int, _ *http.Request) (*http.Response, error) {
		if attempt == 0 {
			return nil, errDropped
		}

		return reply(200, `{"text":" Oui."}`), nil
	}}

	text, err := whisperFor(s, "fr").Transcribe(context.Background(), []float32{0, 0.5}, 16000)
	if err != nil || text != "Oui." {
		t.Fatalf("got %q, %v; want the second attempt's Oui.", text, err)
	}

	if len(s.bodies) != 2 || !bytes.Equal(s.bodies[0], s.bodies[1]) {
		t.Errorf("the phrase was sent %d times, want twice and the same both times", len(s.bodies))
	}
}

func TestWhisperGivesUpWhenTheSecondAttemptGetsNoAnswerEither(t *testing.T) {
	s := &server{answer: func(int, *http.Request) (*http.Response, error) { return nil, errDropped }}

	_, err := whisperFor(s, "fr").Transcribe(context.Background(), []float32{0}, 16000)
	if !errors.Is(err, ErrNoAnswer) || !errors.Is(err, errDropped) {
		t.Errorf("got %v, want ErrNoAnswer carrying the dropped connection", err)
	}

	if len(s.requests) != 2 {
		t.Errorf("the server was asked %d times, want twice", len(s.requests))
	}
}

func TestWhisperDoesNotAskASlowServerTwice(t *testing.T) {
	s := &server{answer: func(_ int, r *http.Request) (*http.Response, error) {
		<-r.Context().Done()

		return nil, r.Context().Err()
	}}
	w := NewWhisper(Config{URL: inference, Language: "fr"}, &http.Client{Transport: s, Timeout: 20 * time.Millisecond})

	_, err := w.Transcribe(context.Background(), []float32{0}, 16000)
	if !errors.Is(err, ErrNoAnswer) {
		t.Errorf("got %v, want ErrNoAnswer", err)
	}

	if len(s.requests) != 1 {
		t.Errorf("a server too slow to answer was asked %d times, want once", len(s.requests))
	}
}

func TestWhisperStopsAskingOnceTheCallerGivesUp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &server{answer: func(int, *http.Request) (*http.Response, error) {
		cancel()

		return nil, errDropped
	}}

	_, err := whisperFor(s, "fr").Transcribe(ctx, []float32{0}, 16000)
	if !errors.Is(err, ErrNoAnswer) {
		t.Errorf("got %v, want ErrNoAnswer", err)
	}

	if len(s.requests) != 1 {
		t.Errorf("the server was asked %d times after the caller gave up, want once", len(s.requests))
	}
}
