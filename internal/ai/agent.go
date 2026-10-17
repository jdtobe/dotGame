package ai

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/jdtobe/dotGame/internal/game"
)

// Agent combines tactical filtering, UCB search, and the learned evaluator.
type Agent struct {
	Model  *Model
	Random *rand.Rand
}

func New(model *Model) *Agent {
	return &Agent{Model: model, Random: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func budget(level string) int {
	switch level {
	case "easy":
		return 8
	case "medium":
		return 60
	case "hard":
		return 180
	case "training":
		return 16
	default:
		return 320
	}
}

// Choose returns a legal move or false when the board is complete/cancelled.
func (a *Agent) Choose(ctx context.Context, s *game.State, level string) (game.Move, bool) {
	iterationsUsed := 0
	if a.Model != nil {
		defer func() { a.Model.ObserveSearch(iterationsUsed) }()
	}
	moves := s.LegalMoves()
	if len(moves) == 0 {
		return game.Move{}, false
	}
	// Capturing is nearly always preferable and dramatically reduces search.
	var captures []game.Move
	for _, m := range moves {
		if wouldCapture(s, m) {
			captures = append(captures, m)
		}
	}
	if len(captures) > 0 {
		moves = captures
	}
	if level == "easy" && a.Random.Intn(3) == 0 {
		return moves[a.Random.Intn(len(moves))], true
	}
	// Avoid creating a third side while safe moves remain.
	var safe []game.Move
	for _, m := range moves {
		if isSafe(s, m) {
			safe = append(safe, m)
		}
	}
	if len(captures) == 0 && len(safe) > 0 {
		moves = safe
	}
	if len(moves) == 1 {
		return moves[0], true
	}
	n := make([]int, len(moves))
	value := make([]float64, len(moves))
	root := s.Current
	iterations := budget(level)
	for i := 0; i < iterations; i++ {
		select {
		case <-ctx.Done():
			return bestMove(moves, n, value), true
		default:
		}
		j := selectUCB(n, value, i)
		c := s.Clone()
		_, _ = c.Apply(moves[j])
		reward := a.rollout(c, root, level)
		n[j]++
		value[j] += reward
		iterationsUsed++
	}
	return bestMove(moves, n, value), true
}

func selectUCB(n []int, v []float64, total int) int {
	best := 0
	score := -math.MaxFloat64
	for i := range n {
		if n[i] == 0 {
			return i
		}
		x := v[i]/float64(n[i]) + 1.25*math.Sqrt(math.Log(float64(total+1))/float64(n[i]))
		if x > score {
			score = x
			best = i
		}
	}
	return best
}
func bestMove(m []game.Move, n []int, v []float64) game.Move {
	best := 0
	score := -math.MaxFloat64
	for i := range m {
		if n[i] > 0 && v[i]/float64(n[i]) > score {
			score = v[i] / float64(n[i])
			best = i
		}
	}
	return m[best]
}

func (a *Agent) rollout(s *game.State, root int, level string) float64 {
	// Full random-tactical playouts are affordable late in the game. Early on,
	// cap depth and use the learned value function.
	maxDepth := 52
	if level == "training" {
		maxDepth = 28
	}
	for depth := 0; !s.Done() && depth < maxDepth; depth++ {
		moves := s.LegalMoves()
		var captures, safe []game.Move
		for _, m := range moves {
			if wouldCapture(s, m) {
				captures = append(captures, m)
			} else if isSafe(s, m) {
				safe = append(safe, m)
			}
		}
		pick := moves
		if len(captures) > 0 {
			pick = captures
		} else if len(safe) > 0 {
			pick = safe
		}
		_, _ = s.Apply(pick[a.Random.Intn(len(pick))])
	}
	if !s.Done() && a.Model != nil {
		return a.Model.Evaluate(s, root)
	}
	best := 0
	for _, v := range s.Scores {
		if v > best {
			best = v
		}
	}
	if s.Scores[root] == best {
		return 1
	}
	return -1
}

func isSafe(s *game.State, m game.Move) bool {
	if m.Orientation == game.Horizontal {
		for _, r := range []int{m.Row - 1, m.Row} {
			if r >= 0 && r < s.Height && s.BoxSides(r, m.Col) == 2 {
				return false
			}
		}
	} else {
		for _, col := range []int{m.Col - 1, m.Col} {
			if col >= 0 && col < s.Width && s.BoxSides(m.Row, col) == 2 {
				return false
			}
		}
	}
	return true
}

func wouldCapture(s *game.State, m game.Move) bool {
	if m.Orientation == game.Horizontal {
		for _, r := range []int{m.Row - 1, m.Row} {
			if r >= 0 && r < s.Height && s.BoxSides(r, m.Col) == 3 {
				return true
			}
		}
		return false
	}
	for _, col := range []int{m.Col - 1, m.Col} {
		if col >= 0 && col < s.Width && s.BoxSides(m.Row, col) == 3 {
			return true
		}
	}
	return false
}
