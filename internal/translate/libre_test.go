package translate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/post/posttest"
)

const address = "http://libre.test/translate"

var errDropped = errors.New("connection reset by peer")

func libreFor(s *posttest.Server, source string) *Libre {
	return NewLibre(Config{URL: address, Source: source, Target: "ru"}, s.Client())
}

func TestLibreAsksForTheTextFromItsLanguageIntoTheTarget(t *testing.T) {
	s := &posttest.Server{Answer: posttest.Answering(200, `{"translatedText":"Да."}`)}

	if _, err := libreFor(s, "fr").Translate(context.Background(), "Oui, l'article 3."); err != nil {
		t.Fatal(err)
	}

	r := s.Requests[0]
	if r.URL.String() != address || r.Header.Get("Content-Type") != "application/json" {
		t.Errorf("got %s as %q, want %s as application/json", r.URL, r.Header.Get("Content-Type"), address)
	}

	var asked map[string]string
	if err := json.Unmarshal(s.Bodies[0], &asked); err != nil {
		t.Fatalf("the request %q is no json object of strings: %v", s.Bodies[0], err)
	}

	want := map[string]string{"q": "Oui, l'article 3.", "source": "fr", "target": "ru", "format": "text"}
	if len(asked) != len(want) {
		t.Errorf("the request holds %v, want exactly %v", asked, want)
	}

	for name, value := range want {
		if asked[name] != value {
			t.Errorf("field %s is %q, want %q", name, asked[name], value)
		}
	}
}

func TestLibreHandsBackTheTranslationTrimmed(t *testing.T) {
	answers := map[string]string{
		`{"translatedText":" Здравствуйте, спасибо, что пришли. "}`:                       "Здравствуйте, спасибо, что пришли.",
		`{"detectedLanguage":{"confidence":90.0,"language":"fr"},"translatedText":"Да."}`: "Да.",
		`{"translatedText":"Срок \u2014 следующая пятница."}`:                             "Срок — следующая пятница.",
	}

	for answer, want := range answers {
		got, err := libreFor(&posttest.Server{Answer: posttest.Answering(200, answer)}, "auto").Translate(context.Background(), "x")
		if err != nil || got != want {
			t.Errorf("%s gave %q, %v; want %q", answer, got, err, want)
		}
	}
}

func TestLibrePassesOnWhyNoTranslationCame(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{400, `{"error":"de is not supported"}`, "400 Bad Request: de is not supported"},
		{429, `{"error":"Slowdown: 30 per 1 minute"}`, "429 Too Many Requests: Slowdown: 30 per 1 minute"},
		{500, `{"error":"Cannot translate text: boom"}`, "500 Internal Server Error: Cannot translate text: boom"},
		{200, `{"detectedLanguage":{"confidence":0,"language":"en"}}`, "200 OK"},
	}

	for _, c := range cases {
		s := &posttest.Server{Answer: posttest.Answering(c.status, c.body)}

		_, err := libreFor(s, "fr").Translate(context.Background(), "Oui.")
		if !errors.Is(err, ErrBadAnswer) || !strings.HasSuffix(err.Error(), ": "+c.want) {
			t.Errorf("%d %s gave %v, want ErrBadAnswer ending in %q", c.status, c.body, err, c.want)
		}
	}
}

func TestLibreCallsAServerThatNeverAnsweredNoAnswer(t *testing.T) {
	s := &posttest.Server{Answer: func(int, *http.Request) (*http.Response, error) { return nil, errDropped }}

	_, err := libreFor(s, "fr").Translate(context.Background(), "Oui.")
	if !errors.Is(err, ErrNoAnswer) || !errors.Is(err, errDropped) {
		t.Errorf("got %v, want ErrNoAnswer carrying the dropped connection", err)
	}
}
