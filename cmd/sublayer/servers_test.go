package main

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/maksim-miliutin/Sublayer/internal/servers"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
)

func rooted(parts ...string) string {
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		root = `C:\`
	}

	return filepath.Join(append([]string{root}, parts...)...)
}

func TestBesideTakesARelativePathFromSublayersFolder(t *testing.T) {
	dir := rooted("Sublayer")
	cases := map[string]string{
		"":                                  "",
		"whisper-server.exe":                filepath.Join(dir, "whisper-server.exe"),
		"models/ggml-small.bin":             filepath.Join(dir, "models", "ggml-small.bin"),
		rooted("whisper", "whisper-server"): rooted("whisper", "whisper-server"),
	}

	for path, want := range cases {
		if got := beside(dir, path); got != want {
			t.Errorf("beside(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestServersForStartWhisperWithItsModelAndLibreTranslateWithEveryLanguage(t *testing.T) {
	dir := rooted("Sublayer")
	s := settings.Defaults()
	s.Whisper.Address = "127.0.0.1:9000"
	s.Libre.Program = "lt-venv/Scripts/libretranslate.exe"

	want := []servers.Server{
		{
			Name:    "whisper-server",
			Address: "127.0.0.1:9000",
			Program: filepath.Join(dir, "whisper-server.exe"),
			Args:    []string{"-m", filepath.Join(dir, "models", "ggml-large-v3-turbo.bin"), "--host", "127.0.0.1", "--port", "9000", "-t", "6"},
		},
		{
			Name:    "LibreTranslate",
			Address: "127.0.0.1:5000",
			Program: filepath.Join(dir, "lt-venv", "Scripts", "libretranslate.exe"),
			Args:    []string{"--host", "127.0.0.1", "--port", "5000", "--load-only", "fr,en,ru"},
		},
	}

	got := serversFor(s, dir, 6)
	if len(got) != len(want) {
		t.Fatalf("got %d servers, want %d", len(got), len(want))
	}

	for i := range want {
		g, w := got[i], want[i]
		if g.Name != w.Name || g.Address != w.Address || g.Program != w.Program || !slices.Equal(g.Args, w.Args) {
			t.Errorf("server %d is %+v, want %+v", i, g, w)
		}
	}
}

func TestServersForLeaveAServerWithoutAProgramToYou(t *testing.T) {
	s := settings.Defaults()
	s.Whisper.Program = ""

	for _, server := range serversFor(s, rooted("Sublayer"), 4) {
		if server.Program != "" {
			t.Errorf("%s is started with %q, want no program", server.Name, server.Program)
		}
	}
}
