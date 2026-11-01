package posttest

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Server struct {
	Answer   func(attempt int, r *http.Request) (*http.Response, error)
	Requests []*http.Request
	Bodies   [][]byte
}

func (s *Server) RoundTrip(r *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}

	s.Requests = append(s.Requests, r)
	s.Bodies = append(s.Bodies, body)

	return s.Answer(len(s.Requests)-1, r)
}

func (s *Server) Client() *http.Client {
	return &http.Client{Transport: s}
}

func Reply(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func Answering(status int, body string) func(int, *http.Request) (*http.Response, error) {
	return func(int, *http.Request) (*http.Response, error) {
		return Reply(status, body), nil
	}
}
