// Package ai provides tactical, search, and continually learned opponents.
package ai

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/jdtobe/dotGame/internal/config"
	"github.com/jdtobe/dotGame/internal/game"
)

// Model is a compact board-size-independent value function. It is deliberately
// small enough to train continuously on a home CPU.
type Model struct {
	Version   int          `json:"version"`
	Input     int          `json:"input"`
	Hidden    int          `json:"hidden"`
	W1        []float64    `json:"w1"`
	B1        []float64    `json:"b1"`
	W2        []float64    `json:"w2"`
	B2        float64      `json:"b2"`
	Games     int          `json:"games"`
	Positions int          `json:"positions"`
	Decisions int          `json:"decisions"`
	Searches  int          `json:"searchIterations"`
	mu        sync.RWMutex `json:"-"`
}

const (
	modelVersion = 2
	inputSize    = 6
	hiddenSize   = 12
)

func NewModel() *Model {
	m := &Model{Version: modelVersion, Input: inputSize, Hidden: hiddenSize, W1: make([]float64, inputSize*hiddenSize), B1: make([]float64, hiddenSize), W2: make([]float64, hiddenSize)}
	// Deterministic small weights make clean installs reproducible while
	// breaking symmetry between hidden units.
	for i := range m.W1 {
		m.W1[i] = math.Sin(float64(i+1)*1.618) * .08
	}
	for i := range m.W2 {
		m.W2[i] = math.Cos(float64(i+1)*.73) * .08
	}
	return m
}

func LoadModel() (*Model, error) {
	d, err := config.Dir()
	if err != nil {
		return NewModel(), err
	}
	b, err := os.ReadFile(filepath.Join(d, "ai-model.json"))
	if os.IsNotExist(err) {
		return NewModel(), nil
	}
	if err != nil {
		return NewModel(), err
	}
	m := NewModel()
	if err = json.Unmarshal(b, m); err != nil || m.Version != modelVersion || m.Input != inputSize || m.Hidden != hiddenSize || len(m.W1) != inputSize*hiddenSize || len(m.B1) != hiddenSize || len(m.W2) != hiddenSize {
		return NewModel(), err
	}
	return m, nil
}

func (m *Model) Save() error {
	m.mu.RLock()
	b, err := json.MarshalIndent(m, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	d, err := config.Dir()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	p := filepath.Join(d, "ai-model.json")
	tmp := p + ".tmp"
	if err = os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func features(s *game.State, player int) []float64 {
	total := float64(s.Width * s.Height)
	if total == 0 {
		total = 1
	}
	claimable, danger := 0., 0.
	for r := 0; r < s.Height; r++ {
		for c := 0; c < s.Width; c++ {
			switch s.BoxSides(r, c) {
			case 3:
				claimable++
			case 2:
				danger++
			}
		}
	}
	bestOther := 0
	for i, v := range s.Scores {
		if i != player && v > bestOther {
			bestOther = v
		}
	}
	progress := 1 - float64(s.Remaining())/float64((s.Height+1)*s.Width+s.Height*(s.Width+1))
	turn := 0.
	if s.Current == player {
		turn = 1
	}
	return []float64{1, (float64(s.Scores[player] - bestOther)) / total, claimable / total, danger / total, progress, turn}
}

// Evaluate estimates a player's eventual result in [-1,1].
func (m *Model) Evaluate(s *game.State, player int) float64 {
	f := features(s, player)
	m.mu.RLock()
	_, v := m.forwardLocked(f)
	m.mu.RUnlock()
	return v
}

// Learn applies one online gradient update for every observed position.
func (m *Model) Learn(states []*game.State, finalScores []int) {
	if len(states) == 0 {
		return
	}
	maxScore := 0
	for _, v := range finalScores {
		if v > maxScore {
			maxScore = v
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	lr := .018 / math.Sqrt(1+float64(m.Games)/40)
	for _, s := range states {
		p := s.Current
		target := -1.
		if finalScores[p] == maxScore {
			target = 1
		}
		f := features(s, p)
		hidden, pred := m.forwardLocked(f)
		outputDelta := (target - pred) * (1 - pred*pred)
		oldW2 := append([]float64(nil), m.W2...)
		for j := 0; j < m.Hidden; j++ {
			m.W2[j] += lr * outputDelta * hidden[j]
		}
		m.B2 += lr * outputDelta
		for j := 0; j < m.Hidden; j++ {
			hiddenDelta := outputDelta * oldW2[j] * (1 - hidden[j]*hidden[j])
			m.B1[j] += lr * hiddenDelta
			for i := 0; i < m.Input; i++ {
				m.W1[j*m.Input+i] += lr * hiddenDelta * f[i]
			}
		}
		m.Positions++
	}
	m.Games++
}

func (m *Model) forwardLocked(input []float64) ([]float64, float64) {
	hidden := make([]float64, m.Hidden)
	for j := 0; j < m.Hidden; j++ {
		v := m.B1[j]
		for i := 0; i < m.Input; i++ {
			v += m.W1[j*m.Input+i] * input[i]
		}
		hidden[j] = math.Tanh(v)
	}
	out := m.B2
	for j := range hidden {
		out += m.W2[j] * hidden[j]
	}
	return hidden, math.Tanh(out)
}

// Snapshot returns counters for the statistics UI.
func (m *Model) Snapshot() (games, positions int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Games, m.Positions
}

// ObserveSearch records aggregate search work for the AI capability display.
func (m *Model) ObserveSearch(iterations int) {
	m.mu.Lock()
	m.Decisions++
	m.Searches += iterations
	m.mu.Unlock()
}

// SearchMetrics returns persistent decision and simulation counters.
func (m *Model) SearchMetrics() (decisions, iterations int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Decisions, m.Searches
}
