package game

import "testing"

func TestCompletingBoxGrantsExtraTurn(t *testing.T) {
	s, _ := New(2, 2, 2)
	for _, m := range []Move{{Horizontal, 0, 0}, {Vertical, 0, 0}, {Vertical, 0, 1}} {
		if _, err := s.Apply(m); err != nil {
			t.Fatal(err)
		}
	}
	// Force the same player for a focused rules test.
	s.Current = 1
	turn, err := s.Apply(Move{Horizontal, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if !turn.ExtraTurn || len(turn.Completed) != 1 || s.Scores[1] != 1 || s.Current != 1 {
		t.Fatalf("unexpected completion: %#v state=%#v", turn, s)
	}
}

func TestDoubleBoxCompletion(t *testing.T) {
	s, _ := New(2, 2, 2)
	// The middle horizontal edge is shared by vertically adjacent boxes.
	edges := []Move{{Horizontal, 0, 0}, {Vertical, 0, 0}, {Vertical, 0, 1}, {Horizontal, 2, 0}, {Vertical, 1, 0}, {Vertical, 1, 1}}
	for _, m := range edges {
		if _, err := s.Apply(m); err != nil {
			t.Fatal(err)
		}
	}
	s.Current = 0
	turn, err := s.Apply(Move{Horizontal, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(turn.Completed) != 2 || s.Scores[0] != 2 {
		t.Fatalf("expected double-box completion, got %d", len(turn.Completed))
	}
}

func TestCloneIsIndependent(t *testing.T) {
	s, _ := New(3, 2, 3)
	c := s.Clone()
	_, _ = c.Apply(Move{Horizontal, 0, 0})
	if !s.IsLegal(Move{Horizontal, 0, 0}) {
		t.Fatal("clone mutated original")
	}
}
