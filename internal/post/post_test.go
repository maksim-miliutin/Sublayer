package post

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/post/posttest"
)

const address = "http://server.test/answer"

var errDropped = errors.New("connection reset by peer")

func question() Request {
	return Request{URL: address, ContentType: "application/json", Body: []byte(`{"q":"Oui."}`)}
}

func TestSendPostsTheBodyAsItsContentType(t *testing.T) {
	s := &posttest.Server{Answer: posttest.Answering(200, "Да.")}

	answer, err := Send(context.Background(), s.Client(), question())
	if err != nil {
		t.Fatal(err)
	}

	r := s.Requests[0]
	if r.Method != http.MethodPost || r.URL.String() != address || r.Header.Get("Content-Type") != "application/json" {
		t.Errorf("got %s %s as %q, want POST %s as application/json", r.Method, r.URL, r.Header.Get("Content-Type"), address)
	}

	if string(s.Bodies[0]) != `{"q":"Oui."}` {
		t.Errorf("the server got %q, want the body as given", s.Bodies[0])
	}

	if answer.Code != 200 || answer.Status != "200 OK" || string(answer.Body) != "Да." {
		t.Errorf("got %d %q %q, want 200, 200 OK and the server's Да.", answer.Code, answer.Status, answer.Body)
	}
}

func TestSendHandsBackAnErrorAnswerWithoutAskingAgain(t *testing.T) {
	s := &posttest.Server{Answer: posttest.Answering(500, `{"error":"failed"}`)}

	answer, err := Send(context.Background(), s.Client(), question())
	if err != nil || answer.Code != 500 || string(answer.Body) != `{"error":"failed"}` {
		t.Errorf("got %d %q, %v; want the 500 as it came", answer.Code, answer.Body, err)
	}

	if len(s.Requests) != 1 {
		t.Errorf("a server that answered was asked %d times, want once", len(s.Requests))
	}
}

func TestSendReadsNoMoreThanTheAnswerLimit(t *testing.T) {
	s := &posttest.Server{Answer: posttest.Answering(200, strings.Repeat("x", answerLimit+1000))}

	answer, err := Send(context.Background(), s.Client(), question())
	if err != nil || len(answer.Body) != answerLimit {
		t.Errorf("got %d bytes, %v; want the first %d", len(answer.Body), err, answerLimit)
	}
}

func TestSendAsksOnceMoreWhenTheFirstAttemptGotNoAnswer(t *testing.T) {
	s := &posttest.Server{Answer: func(attempt int, _ *http.Request) (*http.Response, error) {
		if attempt == 0 {
			return nil, errDropped
		}

		return posttest.Reply(200, "Да."), nil
	}}

	answer, err := Send(context.Background(), s.Client(), question())
	if err != nil || string(answer.Body) != "Да." {
		t.Fatalf("got %q, %v; want the second attempt's answer", answer.Body, err)
	}

	if len(s.Bodies) != 2 || !bytes.Equal(s.Bodies[0], s.Bodies[1]) {
		t.Errorf("the body was sent %d times, want twice and the same both times", len(s.Bodies))
	}
}

func TestSendGivesUpWhenTheSecondAttemptGetsNoAnswerEither(t *testing.T) {
	s := &posttest.Server{Answer: func(int, *http.Request) (*http.Response, error) { return nil, errDropped }}

	if _, err := Send(context.Background(), s.Client(), question()); !errors.Is(err, errDropped) {
		t.Errorf("got %v, want the dropped connection", err)
	}

	if len(s.Requests) != 2 {
		t.Errorf("the server was asked %d times, want twice", len(s.Requests))
	}
}

func TestSendDoesNotAskASlowServerTwice(t *testing.T) {
	s := &posttest.Server{Answer: func(_ int, r *http.Request) (*http.Response, error) {
		<-r.Context().Done()

		return nil, r.Context().Err()
	}}
	client := &http.Client{Transport: s, Timeout: 20 * time.Millisecond}

	if _, err := Send(context.Background(), client, question()); err == nil {
		t.Error("a server that never answered gave no error")
	}

	if len(s.Requests) != 1 {
		t.Errorf("a server too slow to answer was asked %d times, want once", len(s.Requests))
	}
}

func TestSendStopsAskingOnceTheCallerGivesUp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &posttest.Server{Answer: func(int, *http.Request) (*http.Response, error) {
		cancel()

		return nil, errDropped
	}}

	if _, err := Send(ctx, s.Client(), question()); err == nil {
		t.Error("a lost answer gave no error")
	}

	if len(s.Requests) != 1 {
		t.Errorf("the server was asked %d times after the caller gave up, want once", len(s.Requests))
	}
}

func TestDecodeReadsTheJSONOfAPlainAnswer(t *testing.T) {
	var got struct {
		Text string `json:"text"`
	}

	if err := Decode(Answer{Status: "200 OK", Code: 200, Body: []byte(`{"text":"Oui."}`)}, &got); err != nil || got.Text != "Oui." {
		t.Errorf("got %q, %v; want Oui.", got.Text, err)
	}
}

func TestDecodeTellsWhatTheServerSaidInstead(t *testing.T) {
	cases := []struct {
		answer Answer
		want   string
	}{
		{Answer{Status: "400 Bad Request", Code: 400, Body: []byte(`{"error":"fr is not supported"}`)}, "400 Bad Request: fr is not supported"},
		{Answer{Status: "200 OK", Code: 200, Body: []byte(`{"error":"busy"}`)}, "200 OK: busy"},
		{Answer{Status: "404 Not Found", Code: 404, Body: []byte("File Not Found (/x)")}, "404 Not Found: File Not Found (/x)"},
		{Answer{Status: "200 OK", Code: 200, Body: []byte(" Oui.\n")}, "200 OK: Oui."},
		{Answer{Status: "500 Internal Server Error", Code: 500, Body: []byte(`{"text":"Oui."}`)}, "500 Internal Server Error"},
	}

	for _, c := range cases {
		var got struct {
			Text string `json:"text"`
		}

		if err := Decode(c.answer, &got); err == nil || err.Error() != c.want {
			t.Errorf("%s %q gave %v, want %q", c.answer.Status, c.answer.Body, err, c.want)
		}
	}
}
