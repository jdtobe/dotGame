package tui

import (
	"testing"

	"github.com/jdtobe/dotGame/internal/config"
)

func TestStartActionIsReachable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := model{settings: config.Defaults()}
	m.cursor = m.rows() - 3
	updated, _ := m.activate()
	got := updated.(model)
	if !got.start || got.watch || got.train {
		t.Fatalf("start action was not selected: %#v", got)
	}
}

func TestWatchActionDoesNotOverwritePlayers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := model{settings: config.Defaults()}
	m.cursor = m.rows() - 2
	updated, _ := m.activate()
	got := updated.(model)
	if !got.watch || got.settings.Players[0].Kind != config.Human {
		t.Fatalf("watch action changed saved player configuration: %#v", got)
	}
}
