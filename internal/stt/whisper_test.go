package stt

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/post/posttest"
	"github.com/maksim-miliutin/Sublayer/internal/wav"
)

const inference = "http://whisper.test/inference"

var errDropped = errors.New("connection reset by peer")

func whisperFor(s *posttest.Server, language string) *Whisper {
	return NewWhisper(Config{URL: inference, Language: language}, s.Client())
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
	s := &posttest.Server{Answer: posttest.Answering(200, `{"text":" Oui."}`)}
	speech := []float32{0, 0.25, -0.5, 0.75, -1}

	if _, err := whisperFor(s, "fr").Transcribe(context.Background(), speech, 22050); err != nil {
		t.Fatal(err)
	}

	r := s.Requests[0]
	if r.Method != http.MethodPost || r.URL.String() != inference {
		t.Fatalf("got %s %s, want POST %s", r.Method, r.URL, inference)
	}

	fields := formFields(t, r, s.Bodies[0])
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
		got, err := whisperFor(&posttest.Server{Answer: posttest.Answering(200, answer)}, "fr").Transcribe(context.Background(), []float32{0}, 16000)
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
		s := &posttest.Server{Answer: posttest.Answering(c.status, c.body)}

		_, err := whisperFor(s, "fr").Transcribe(context.Background(), []float32{0}, 16000)
		if !errors.Is(err, ErrBadAnswer) || !strings.HasSuffix(err.Error(), ": "+c.want) {
			t.Errorf("%d %q gave %v, want ErrBadAnswer ending in %q", c.status, c.body, err, c.want)
		}

		if len(s.Requests) != 1 {
			t.Errorf("%d %q was asked %d times, want once", c.status, c.body, len(s.Requests))
		}
	}
}

func TestWhisperCallsAServerThatNeverAnsweredNoAnswer(t *testing.T) {
	s := &posttest.Server{Answer: func(int, *http.Request) (*http.Response, error) { return nil, errDropped }}

	_, err := whisperFor(s, "fr").Transcribe(context.Background(), []float32{0}, 16000)
	if !errors.Is(err, ErrNoAnswer) || !errors.Is(err, errDropped) {
		t.Errorf("got %v, want ErrNoAnswer carrying the dropped connection", err)
	}
}
