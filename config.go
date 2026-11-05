package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// config holds the defaults used by say when a flag is not given.
type config struct {
	Lang  string  `json:"lang"`
	Voice string  `json:"voice"`
	Speed float64 `json:"speed"`
}

var builtinConfig = config{Lang: "it_IT", Voice: "paola-medium", Speed: 1.0}

// configPath is $XDG_CONFIG_HOME/titties/config.json or ~/.config/titties/config.json.
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "titties", "config.json"), nil
}

// loadConfig returns the saved defaults, falling back to the built-in ones for anything unset.
func loadConfig() (config, error) {
	c := builtinConfig
	path, err := configPath()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	var saved config
	if err := json.Unmarshal(data, &saved); err != nil {
		return c, err
	}
	if saved.Lang != "" {
		c.Lang = saved.Lang
	}
	if saved.Voice != "" {
		c.Voice = saved.Voice
	}
	if saved.Speed > 0 {
		c.Speed = saved.Speed
	}
	return c, nil
}

func saveConfig(c config) (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(data, '\n'), 0o644) //nolint:gosec // not secret
}
