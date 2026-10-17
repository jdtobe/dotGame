// Package config loads and atomically saves user preferences and statistics.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const currentVersion = 1

type PlayerKind string

const (
	Human PlayerKind = "human"
	AI    PlayerKind = "ai"
)

// Player describes one seat at the table.
type Player struct {
	Name       string     `json:"name"`
	Kind       PlayerKind `json:"kind"`
	Difficulty string     `json:"difficulty,omitempty"`
}

// Settings contains persistent, user-selectable options.
type Settings struct {
	Version    int      `json:"version"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Theme      string   `json:"theme"`
	Animations bool     `json:"animations"`
	Players    []Player `json:"players"`
}

// Stats holds lifetime aggregate statistics.
type Stats struct {
	Version    int            `json:"version"`
	Games      int            `json:"games"`
	Ties       int            `json:"ties"`
	TotalMoves int            `json:"totalMoves"`
	Boxes      int            `json:"boxes"`
	Wins       map[string]int `json:"wins"`
}

func Defaults() Settings {
	return Settings{Version: currentVersion, Width: 10, Height: 10, Theme: "modern", Animations: true, Players: []Player{{"You", Human, ""}, {"Ada", AI, "adaptive"}}}
}

// Dir returns DotGame's per-user data directory.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "dotgame"), nil
}

func path(name string) (string, error) {
	d, e := Dir()
	if e != nil {
		return "", e
	}
	return filepath.Join(d, name), nil
}

func load(name string, dst any) error {
	p, err := path(name)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func save(name string, src any) error {
	p, err := path(name)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err = os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func LoadSettings() (Settings, error) {
	s := Defaults()
	if err := load("settings.json", &s); err != nil {
		return Defaults(), fmt.Errorf("load settings: %w", err)
	}
	if s.Version != currentVersion || s.Width < 2 || s.Height < 2 || len(s.Players) < 2 || len(s.Players) > 4 {
		return Defaults(), nil
	}
	return s, nil
}
func SaveSettings(s Settings) error { s.Version = currentVersion; return save("settings.json", s) }

func LoadStats() (Stats, error) {
	s := Stats{Version: currentVersion, Wins: map[string]int{}}
	err := load("stats.json", &s)
	if s.Wins == nil {
		s.Wins = map[string]int{}
	}
	return s, err
}
func SaveStats(s Stats) error { s.Version = currentVersion; return save("stats.json", s) }
