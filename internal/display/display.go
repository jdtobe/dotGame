// Package display renders and operates a match with Ebitengine.
package display

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/jdtobe/dotGame/internal/ai"
	"github.com/jdtobe/dotGame/internal/config"
	"github.com/jdtobe/dotGame/internal/game"
	"github.com/jdtobe/dotGame/internal/theme"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

const screenW, screenH = 1200, 820

var (
	regularFont = mustFont(goregular.TTF)
	boldFont    = mustFont(gobold.TTF)
)

func mustFont(data []byte) *text.GoTextFaceSource {
	source, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	return source
}

func drawText(dst *ebiten.Image, value string, x, y, size float64, clr color.Color, bold bool) {
	source := regularFont
	if bold {
		source = boldFont
	}
	face := &text.GoTextFace{Source: source, Size: size}
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(clr)
	op.LineSpacing = size * 1.35
	text.Draw(dst, value, face, op)
}

type aiResult struct {
	move game.Move
	ok   bool
}

type captureAnimation struct {
	start time.Time
}

// Match implements ebiten.Game.
type Match struct {
	state             *game.State
	settings          config.Settings
	stats             config.Stats
	model             *ai.Model
	agent             *ai.Agent
	aiThinking        bool
	aiResult          chan aiResult
	trajectory        []*game.State
	learned           bool
	status            string
	lastMove          game.Move
	hasLastMove       bool
	lastMoveAt        time.Time
	captureAnimations map[int]captureAnimation
	nextCaptureAt     time.Time
	gameOverAt        time.Time
	characters        []characterSprite
	startedAt         time.Time
	statusAt          time.Time
	scoreBumpAt       [4]time.Time
	previousTurn      int
	turnChangedAt     time.Time
	boardLayer        *ebiten.Image
	boardLayerKey     boardLayerKey
	boardLayerValid   bool
}

// New creates a graphical match from validated settings.
func New(settings config.Settings, stats config.Stats, model *ai.Model) (*Match, error) {
	s, err := game.New(settings.Width, settings.Height, len(settings.Players))
	if err != nil {
		return nil, err
	}
	characters, err := loadCharacterSprites()
	if err != nil {
		return nil, fmt.Errorf("load celebration characters: %w", err)
	}
	return &Match{state: s, settings: settings, stats: stats, model: model, agent: ai.New(model), aiResult: make(chan aiResult, 1), captureAnimations: map[int]captureAnimation{}, characters: characters, startedAt: time.Now()}, nil
}

func (m *Match) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		p := theme.Next(m.settings.Theme)
		m.settings.Theme = p.ID
		_ = config.SaveSettings(m.settings)
		m.setStatus("Theme: " + p.Name)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		m.settings.Animations = !m.settings.Animations
		_ = config.SaveSettings(m.settings)
		if m.settings.Animations {
			m.setStatus("Motion: full")
		} else {
			m.setStatus("Motion: reduced")
		}
	}
	if m.state.Done() {
		m.finish()
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyR) {
			m.restart()
		}
		return nil
	}
	select {
	case r := <-m.aiResult:
		m.aiThinking = false
		if r.ok {
			m.apply(r.move)
		}
		return nil
	default:
	}
	p := m.settings.Players[m.state.Current]
	if p.Kind == config.AI {
		if !m.aiThinking {
			m.aiThinking = true
			s := m.state.Clone()
			level := p.Difficulty
			go func() { mv, ok := m.agent.Choose(context.Background(), s, level); m.aiResult <- aiResult{mv, ok} }()
		}
		return nil
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if mv, ok := m.hitMove(float64(x), float64(y)); ok {
			m.apply(mv)
		}
	}
	return nil
}

func (m *Match) apply(mv game.Move) {
	m.trajectory = append(m.trajectory, m.state.Clone())
	t, err := m.state.Apply(mv)
	if err != nil {
		return
	}
	now := time.Now()
	if len(t.Completed) > 0 {
		m.scheduleCaptures(t.Completed, now)
		status := fmt.Sprintf("%s captured %d box", m.settings.Players[t.Player].Name, len(t.Completed))
		if len(t.Completed) > 1 {
			status += "es"
		}
		m.setStatus(status)
		// The score ticks up as the last newly queued box lands.
		m.scoreBumpAt[t.Player] = m.nextCaptureAt.Add(-captureStagger)
	}
	if m.state.Current != t.Player {
		m.previousTurn = t.Player
		m.turnChangedAt = now
	}
	m.lastMove = mv
	m.hasLastMove = true
	m.lastMoveAt = now
}

const captureStagger = 110 * time.Millisecond

func (m *Match) scheduleCaptures(boxes []int, now time.Time) {
	if m.nextCaptureAt.Before(now) {
		m.nextCaptureAt = now
	}
	for _, box := range boxes {
		m.captureAnimations[box] = captureAnimation{start: m.nextCaptureAt}
		m.nextCaptureAt = m.nextCaptureAt.Add(captureStagger)
	}
}

func (m *Match) finish() {
	if m.learned {
		return
	}
	m.learned = true
	m.gameOverAt = time.Now()
	if m.settings.Animations && !m.nextCaptureAt.IsZero() {
		lastCaptureEnd := m.nextCaptureAt.Add(-captureStagger).Add(time.Duration(captureDuration * float64(time.Second)))
		if lastCaptureEnd.After(m.gameOverAt) {
			m.gameOverAt = lastCaptureEnd
		}
	}
	m.model.Learn(m.trajectory, m.state.Scores)
	_ = m.model.Save()
	m.stats.Games++
	m.stats.TotalMoves += len(m.state.History)
	m.stats.Boxes += m.state.Width * m.state.Height
	w := m.state.Winners()
	if len(w) > 1 {
		m.stats.Ties++
	} else if len(w) == 1 {
		m.stats.Wins[m.settings.Players[w[0]].Name]++
	}
	_ = config.SaveStats(m.stats)
}

// restart begins a fresh match with the same seats and board after a
// completed game; results were already recorded by finish.
func (m *Match) restart() {
	s, err := game.New(m.state.Width, m.state.Height, m.state.Players)
	if err != nil {
		return
	}
	m.state = s
	m.trajectory = nil
	m.learned = false
	m.status = ""
	m.hasLastMove = false
	m.captureAnimations = map[int]captureAnimation{}
	m.nextCaptureAt = time.Time{}
	m.gameOverAt = time.Time{}
	m.scoreBumpAt = [4]time.Time{}
	m.previousTurn = 0
	m.turnChangedAt = time.Time{}
	m.startedAt = time.Now()
}

func (m *Match) setStatus(status string) {
	m.status = status
	m.statusAt = time.Now()
}

func (m *Match) geometry() (left, top, step float64) {
	areaW, areaH := 820., 704.
	step = math.Min(areaW/float64(m.state.Width), areaH/float64(m.state.Height))
	left = (900 - step*float64(m.state.Width)) / 2
	top = (screenH - step*float64(m.state.Height)) / 2
	return
}
func (m *Match) hitMove(x, y float64) (game.Move, bool) {
	left, top, step := m.geometry()
	best := game.Move{}
	dist := 10.
	for r := 0; r <= m.state.Height; r++ {
		for c := 0; c < m.state.Width; c++ {
			d := math.Abs(y - (top + float64(r)*step))
			if x >= left+float64(c)*step && x <= left+float64(c+1)*step && d < dist {
				best = game.Move{Orientation: game.Horizontal, Row: r, Col: c}
				dist = d
			}
		}
	}
	for r := 0; r < m.state.Height; r++ {
		for c := 0; c <= m.state.Width; c++ {
			d := math.Abs(x - (left + float64(c)*step))
			if y >= top+float64(r)*step && y <= top+float64(r+1)*step && d < dist {
				best = game.Move{Orientation: game.Vertical, Row: r, Col: c}
				dist = d
			}
		}
	}
	return best, dist < 10 && m.state.IsLegal(best)
}

// Capture timing, in seconds. A box snaps in quickly, settles with one soft
// overshoot, and is fully at rest by captureDuration.
const (
	captureDuration = .42
	capturePop      = .13 // time to reach peak size
	capturePeak     = 1.08
)

// captureScale is the uniform scale of a captured box: a fast ease-out pop
// from half size to a slight overshoot, then a damped settle onto 1.
func captureScale(age float64) float64 {
	if age <= 0 {
		return .5
	}
	if age >= captureDuration {
		return 1
	}
	if age < capturePop {
		t := age / capturePop
		return .5 + (capturePeak-.5)*(1-math.Pow(1-t, 3))
	}
	t := (age - capturePop) / (captureDuration - capturePop)
	return 1 + (capturePeak-1)*math.Cos(t*math.Pi*1.5)*(1-t)*(1-t)
}

func drawCenteredText(dst *ebiten.Image, value string, centerX, y, size float64, clr color.Color, bold bool) {
	source := regularFont
	if bold {
		source = boldFont
	}
	face := &text.GoTextFace{Source: source, Size: size}
	width, _ := text.Measure(value, face, size*1.35)
	drawText(dst, value, centerX-width/2, y, size, clr, bold)
}

func movePoints(mv game.Move, left, top, step float64) (x1, y1, x2, y2 float64) {
	x1, y1 = left+float64(mv.Col)*step, top+float64(mv.Row)*step
	if mv.Orientation == game.Horizontal {
		return x1, y1, x1 + step, y1
	}
	return x1, y1, x1, y1 + step
}
func (m *Match) Layout(_, _ int) (int, int) { return screenW, screenH }

// Run opens the Ebitengine window and blocks until it closes.
func Run(m *Match) error {
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("DotGame")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	return ebiten.RunGame(m)
}
