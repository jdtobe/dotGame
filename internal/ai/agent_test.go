package ai

import (
	"context"
	"testing"

	"github.com/jdtobe/dotGame/internal/game"
)

func TestAgentTakesAvailableBox(t *testing.T) {
	s, _ := game.New(2, 2, 2)
	for _, mv := range []game.Move{{Orientation: game.Horizontal, Row: 0, Col: 0}, {Orientation: game.Vertical, Row: 0, Col: 0}, {Orientation: game.Vertical, Row: 0, Col: 1}} {
		_, _ = s.Apply(mv)
	}
	a := New(NewModel())
	mv, ok := a.Choose(context.Background(), s, "easy")
	if !ok || mv != (game.Move{Orientation: game.Horizontal, Row: 1, Col: 0}) {
		t.Fatalf("agent chose %v instead of immediate capture", mv)
	}
}

func TestLearningObservesEveryPosition(t *testing.T) {
	s, _ := game.New(2, 2, 2)
	states := []*game.State{s.Clone(), s.Clone(), s.Clone()}
	m := NewModel()
	m.Learn(states, []int{4, 0})
	games, positions := m.Snapshot()
	if games != 1 || positions != len(states) {
		t.Fatalf("got games=%d positions=%d", games, positions)
	}
}
