package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/maksim-miliutin/Sublayer/internal/settings"
)

func settingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "Sublayer", "config.json"), nil
}

func loadSettings() (settings.Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return settings.Settings{}, err
	}

	file, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return settings.Defaults(), writeSettings(path, settings.Defaults())
	}
	if err != nil {
		return settings.Settings{}, err
	}

	s, err := settings.Parse(file)
	if err != nil {
		return settings.Settings{}, fmt.Errorf("%s: %w", path, err)
	}

	return s, nil
}

func writeSettings(path string, s settings.Settings) error {
	file, err := settings.Encode(s)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, file, 0o644)
}
