package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/servers"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
)

const (
	serverPatience = 90 * time.Second
	askEvery       = 250 * time.Millisecond
	askTimeout     = 300 * time.Millisecond
)

func startServers(ctx context.Context, s settings.Settings, starting func(servers.Server)) (stop func() error, err error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	running, err := servers.Start(ctx, servers.Config{
		Answers:  answers,
		Start:    servers.Launch,
		Starting: starting,
		Every:    askEvery,
		Patience: serverPatience,
	}, serversFor(s, filepath.Dir(exe), max(1, runtime.NumCPU()/2))...)
	if err != nil {
		return nil, withSettingsPath(err)
	}

	return running.Stop, nil
}

func serveUntilInterrupted(m machine, s settings.Settings) (stop func() error, err error) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	return m.servers(ctx, s, announce)
}

func announce(server servers.Server) {
	fmt.Fprintf(os.Stderr, "starting %s: %s\n", server.Name, server.Program)
}

func withSettingsPath(err error) error {
	path, pathErr := settingsPath()
	if pathErr != nil || errors.Is(err, context.Canceled) {
		return err
	}

	return fmt.Errorf("%w\nthe servers are set in %s", err, path)
}

func answers(address string) bool {
	conn, err := net.DialTimeout("tcp", address, askTimeout)
	if err != nil {
		return false
	}
	conn.Close()

	return true
}

func serversFor(s settings.Settings, dir string, threads int) []servers.Server {
	// The settings checked both addresses when they were read.
	whisperHost, whisperPort, _ := net.SplitHostPort(s.Whisper.Address)
	libreHost, librePort, _ := net.SplitHostPort(s.Libre.Address)

	return []servers.Server{
		{
			Name:    "whisper-server",
			Address: s.Whisper.Address,
			Program: beside(dir, s.Whisper.Program),
			Args:    []string{"-m", beside(dir, s.Whisper.Model), "--host", whisperHost, "--port", whisperPort, "-t", strconv.Itoa(threads)},
		},
		{
			Name:    "LibreTranslate",
			Address: s.Libre.Address,
			Program: beside(dir, s.Libre.Program),
			Args:    []string{"--host", libreHost, "--port", librePort, "--load-only", strings.Join(settings.Codes(), ",")},
		},
	}
}

func beside(dir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(dir, path)
}
