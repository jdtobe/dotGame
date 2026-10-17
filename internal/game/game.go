// Package game implements the UI-independent rules of Dots and Boxes.
package game

import (
	"errors"
	"fmt"
)

// Orientation identifies the direction of an edge.
type Orientation uint8

const (
	Horizontal Orientation = iota
	Vertical
)

// Move identifies one edge by orientation, row, and column.
type Move struct {
	Orientation Orientation `json:"orientation"`
	Row         int         `json:"row"`
	Col         int         `json:"col"`
}

func (m Move) String() string {
	prefix := "H"
	if m.Orientation == Vertical {
		prefix = "V"
	}
	return fmt.Sprintf("%s(%d,%d)", prefix, m.Row, m.Col)
}

// Turn records the result of an applied move.
type Turn struct {
	Player    int   `json:"player"`
	Move      Move  `json:"move"`
	Completed []int `json:"completed,omitempty"`
	ExtraTurn bool  `json:"extraTurn"`
}

var ErrIllegalMove = errors.New("illegal or already claimed edge")

// State is a complete, cloneable game position. Edge values are -1 when free
// and otherwise contain the zero-based owner index.
type State struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Players    int    `json:"players"`
	Current    int    `json:"current"`
	Horizontal []int8 `json:"horizontal"`
	Vertical   []int8 `json:"vertical"`
	Boxes      []int8 `json:"boxes"`
	Scores     []int  `json:"scores"`
	History    []Turn `json:"history,omitempty"`
}

// New returns an empty board. Width and height count boxes, not dots.
func New(width, height, players int) (*State, error) {
	if width < 2 || height < 2 || width > 30 || height > 30 {
		return nil, fmt.Errorf("board dimensions must each be between 2 and 30")
	}
	if players < 2 || players > 4 {
		return nil, fmt.Errorf("player count must be between 2 and 4")
	}
	s := &State{Width: width, Height: height, Players: players, Scores: make([]int, players)}
	s.Horizontal = emptyEdges((height + 1) * width)
	s.Vertical = emptyEdges(height * (width + 1))
	s.Boxes = emptyEdges(width * height)
	return s, nil
}

func emptyEdges(n int) []int8 {
	v := make([]int8, n)
	for i := range v {
		v[i] = -1
	}
	return v
}

// Clone returns a deep copy suitable for AI simulation.
func (s *State) Clone() *State {
	c := *s
	c.Horizontal = append([]int8(nil), s.Horizontal...)
	c.Vertical = append([]int8(nil), s.Vertical...)
	c.Boxes = append([]int8(nil), s.Boxes...)
	c.Scores = append([]int(nil), s.Scores...)
	c.History = append([]Turn(nil), s.History...)
	return &c
}

func (s *State) hIndex(row, col int) int { return row*s.Width + col }
func (s *State) vIndex(row, col int) int { return row*(s.Width+1) + col }
func (s *State) bIndex(row, col int) int { return row*s.Width + col }

// IsLegal reports whether m identifies an unclaimed edge on the board.
func (s *State) IsLegal(m Move) bool {
	switch m.Orientation {
	case Horizontal:
		return m.Row >= 0 && m.Row <= s.Height && m.Col >= 0 && m.Col < s.Width && s.Horizontal[s.hIndex(m.Row, m.Col)] < 0
	case Vertical:
		return m.Row >= 0 && m.Row < s.Height && m.Col >= 0 && m.Col <= s.Width && s.Vertical[s.vIndex(m.Row, m.Col)] < 0
	default:
		return false
	}
}

// Owner returns the owner of m, or -1 when it is free or invalid.
func (s *State) Owner(m Move) int {
	if m.Orientation == Horizontal && m.Row >= 0 && m.Row <= s.Height && m.Col >= 0 && m.Col < s.Width {
		return int(s.Horizontal[s.hIndex(m.Row, m.Col)])
	}
	if m.Orientation == Vertical && m.Row >= 0 && m.Row < s.Height && m.Col >= 0 && m.Col <= s.Width {
		return int(s.Vertical[s.vIndex(m.Row, m.Col)])
	}
	return -1
}

// LegalMoves returns all currently available edges in stable board order.
func (s *State) LegalMoves() []Move {
	moves := make([]Move, 0, s.Remaining())
	for r := 0; r <= s.Height; r++ {
		for c := 0; c < s.Width; c++ {
			m := Move{Horizontal, r, c}
			if s.IsLegal(m) {
				moves = append(moves, m)
			}
		}
	}
	for r := 0; r < s.Height; r++ {
		for c := 0; c <= s.Width; c++ {
			m := Move{Vertical, r, c}
			if s.IsLegal(m) {
				moves = append(moves, m)
			}
		}
	}
	return moves
}

// Apply claims an edge, completes adjacent boxes, and advances the turn unless
// the current player scored.
func (s *State) Apply(m Move) (Turn, error) {
	if !s.IsLegal(m) {
		return Turn{}, ErrIllegalMove
	}
	player := s.Current
	if m.Orientation == Horizontal {
		s.Horizontal[s.hIndex(m.Row, m.Col)] = int8(player)
	} else {
		s.Vertical[s.vIndex(m.Row, m.Col)] = int8(player)
	}
	completed := make([]int, 0, 2)
	check := func(r, c int) {
		if r < 0 || r >= s.Height || c < 0 || c >= s.Width {
			return
		}
		i := s.bIndex(r, c)
		if s.Boxes[i] < 0 && s.boxSides(r, c) == 4 {
			s.Boxes[i] = int8(player)
			s.Scores[player]++
			completed = append(completed, i)
		}
	}
	if m.Orientation == Horizontal {
		check(m.Row-1, m.Col)
		check(m.Row, m.Col)
	} else {
		check(m.Row, m.Col-1)
		check(m.Row, m.Col)
	}
	t := Turn{Player: player, Move: m, Completed: completed, ExtraTurn: len(completed) > 0}
	if !t.ExtraTurn {
		s.Current = (s.Current + 1) % s.Players
	}
	s.History = append(s.History, t)
	return t, nil
}

func (s *State) boxSides(r, c int) int {
	n := 0
	if s.Horizontal[s.hIndex(r, c)] >= 0 {
		n++
	}
	if s.Horizontal[s.hIndex(r+1, c)] >= 0 {
		n++
	}
	if s.Vertical[s.vIndex(r, c)] >= 0 {
		n++
	}
	if s.Vertical[s.vIndex(r, c+1)] >= 0 {
		n++
	}
	return n
}

// BoxSides returns the number of claimed sides around a box.
func (s *State) BoxSides(row, col int) int {
	if row < 0 || row >= s.Height || col < 0 || col >= s.Width {
		return 0
	}
	return s.boxSides(row, col)
}

// BoxOwner returns a box owner, or -1 for an incomplete/invalid box.
func (s *State) BoxOwner(row, col int) int {
	if row < 0 || row >= s.Height || col < 0 || col >= s.Width {
		return -1
	}
	return int(s.Boxes[s.bIndex(row, col)])
}

// Remaining is the number of unclaimed edges.
func (s *State) Remaining() int {
	n := 0
	for _, v := range s.Horizontal {
		if v < 0 {
			n++
		}
	}
	for _, v := range s.Vertical {
		if v < 0 {
			n++
		}
	}
	return n
}

// Done reports whether every edge has been claimed.
func (s *State) Done() bool { return s.Remaining() == 0 }

// Winners returns all players tied for the highest score when the game ends.
func (s *State) Winners() []int {
	if !s.Done() {
		return nil
	}
	best := -1
	var out []int
	for i, v := range s.Scores {
		if v > best {
			best = v
			out = []int{i}
		} else if v == best {
			out = append(out, i)
		}
	}
	return out
}
