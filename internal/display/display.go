// Package display renders and operates a match with Ebitengine.
package display

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
	return &Match{state: s, settings: settings, stats: stats, model: model, agent: ai.New(model), aiResult: make(chan aiResult, 1), captureAnimations: map[int]captureAnimation{}, characters: characters}, nil
}

func (m *Match) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		p := theme.Next(m.settings.Theme)
		m.settings.Theme = p.ID
		_ = config.SaveSettings(m.settings)
		m.status = "Theme: " + p.Name
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		m.settings.Animations = !m.settings.Animations
		_ = config.SaveSettings(m.settings)
		if m.settings.Animations {
			m.status = "Motion: full"
		} else {
			m.status = "Motion: reduced"
		}
	}
	if m.state.Done() {
		m.finish()
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
	if len(t.Completed) > 0 {
		m.scheduleCaptures(t.Completed, time.Now())
		m.status = fmt.Sprintf("%s captured %d box", m.settings.Players[t.Player].Name, len(t.Completed))
		if len(t.Completed) > 1 {
			m.status += "es"
		}
	} else {
		m.status = ""
	}
	m.lastMove = mv
	m.hasLastMove = true
	m.lastMoveAt = time.Now()
}

const captureStagger = 250 * time.Millisecond

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

func (m *Match) geometry() (left, top, step float64) {
	areaW, areaH := 880., 740.
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

func (m *Match) Draw(screen *ebiten.Image) {
	p := theme.ByID(m.settings.Theme)
	screen.Fill(p.Background)
	left, top, step := m.geometry()
	// Claimed boxes sit beneath edges. A newly captured box expands from its
	// center, overshoots, and settles with a short damped jello deformation.
	for r := 0; r < m.state.Height; r++ {
		for c := 0; c < m.state.Width; c++ {
			owner := m.state.BoxOwner(r, c)
			if owner < 0 {
				continue
			}
			col := p.Players[owner]
			col.A = 68
			boxIndex := r*m.state.Width + c
			scaleX, scaleY := 1., 1.
			animation, animated := m.captureAnimations[boxIndex]
			captureAge := 0.
			if m.settings.Animations && animated {
				captureAge = time.Since(animation.start).Seconds()
				if captureAge < 0 {
					scaleX, scaleY, col.A = 0, 0, 0
				} else {
					scaleX, scaleY = jelloScale(captureAge)
					if captureAge < captureDuration {
						col.A = uint8(185 - 100*captureAge/captureDuration)
					}
				}
			}
			sizeX, sizeY := (step-10)*scaleX, (step-10)*scaleY
			insetX, insetY := (step-sizeX)/2, (step-sizeY)/2
			vector.DrawFilledRect(screen, float32(left+float64(c)*step+insetX), float32(top+float64(r)*step+insetY), float32(sizeX), float32(sizeY), col, true)
			if m.settings.Animations && animated && captureAge >= 0 && captureAge < captureDuration {
				outline := p.Players[owner]
				outline.A = uint8(230 * (1 - captureAge/captureDuration))
				vector.StrokeRect(screen, float32(left+float64(c)*step+insetX), float32(top+float64(r)*step+insetY), float32(sizeX), float32(sizeY), 4, outline, true)
			}
		}
	}
	line := func(mv game.Move, x1, y1, x2, y2 float64) {
		owner := m.state.Owner(mv)
		col := p.Line
		width := float32(2)
		if owner >= 0 {
			col = p.Players[owner]
			width = 7
			if m.settings.Animations && m.hasLastMove && mv == m.lastMove {
				progress := math.Min(1, time.Since(m.lastMoveAt).Seconds()/.18)
				midX, midY := (x1+x2)/2, (y1+y2)/2
				dx, dy := (x2-x1)*progress/2, (y2-y1)*progress/2
				x1, y1, x2, y2 = midX-dx, midY-dy, midX+dx, midY+dy
				age := time.Since(m.lastMoveAt).Seconds()
				if age < .55 {
					halo := p.Hover
					halo.A = uint8(150 * (1 - age/.55))
					vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 13, halo, true)
				}
			}
		}
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), width, col, false)
	}
	for r := 0; r <= m.state.Height; r++ {
		for c := 0; c < m.state.Width; c++ {
			line(game.Move{Orientation: game.Horizontal, Row: r, Col: c}, left+float64(c)*step, top+float64(r)*step, left+float64(c+1)*step, top+float64(r)*step)
		}
	}
	for r := 0; r < m.state.Height; r++ {
		for c := 0; c <= m.state.Width; c++ {
			line(game.Move{Orientation: game.Vertical, Row: r, Col: c}, left+float64(c)*step, top+float64(r)*step, left+float64(c)*step, top+float64(r+1)*step)
		}
	}
	if !m.state.Done() && m.settings.Players[m.state.Current].Kind == config.Human {
		x, y := ebiten.CursorPosition()
		if mv, ok := m.hitMove(float64(x), float64(y)); ok {
			x1, y1, x2, y2 := movePoints(mv, left, top, step)
			hover := p.Hover
			hover.A = 205
			vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 6, hover, true)
		}
	}
	dotRadius := float32(math.Min(3.3, step*.08))
	for r := 0; r <= m.state.Height; r++ {
		for c := 0; c <= m.state.Width; c++ {
			vector.DrawFilledCircle(screen, float32(left+float64(c)*step), float32(top+float64(r)*step), dotRadius, p.Dot, true)
		}
	}
	if m.state.Done() {
		m.drawCelebration(screen, p)
	}
	vector.DrawFilledRect(screen, 920, 0, 280, screenH, p.Panel, false)
	drawText(screen, "DOTGAME", 950, 34, 29, p.Text, true)
	drawText(screen, strings.ToUpper(p.Name), 950, 70, 12, p.Accent, true)
	drawText(screen, fmt.Sprintf("BOARD  %d × %d", m.state.Width, m.state.Height), 950, 100, 13, p.Muted, false)
	y := 145
	for i, pl := range m.settings.Players {
		marker := ""
		if i == m.state.Current && !m.state.Done() {
			marker = "CURRENT  "
		}
		if i == m.state.Current && !m.state.Done() {
			active := p.Players[i]
			active.A = 28
			vector.DrawFilledRect(screen, 936, float32(y-8), 238, 56, active, false)
		}
		drawText(screen, marker+strings.ToUpper(pl.Name), 956, float64(y-2), 13, p.Muted, true)
		drawText(screen, fmt.Sprintf("%02d", m.state.Scores[i]), 956, float64(y+18), 25, p.Text, true)
		drawText(screen, "BOXES", 1000, float64(y+26), 10, p.Muted, false)
		vector.DrawFilledRect(screen, 938, float32(y-2), 6, 39, p.Players[i], false)
		y += 68
	}
	if m.aiThinking {
		drawText(screen, "●  AI THINKING", 950, float64(y+10), 12, p.Accent, true)
	}
	if m.status != "" {
		drawText(screen, m.status, 950, float64(y+42), 12, p.Text, false)
	}
	if m.state.Done() {
		drawText(screen, "GAME COMPLETE", 950, float64(y+90), 17, p.Accent, true)
		w := m.state.Winners()
		names := make([]string, len(w))
		for i, id := range w {
			names[i] = m.settings.Players[id].Name
		}
		drawText(screen, "Winner: "+strings.Join(names, " & "), 950, float64(y+120), 13, p.Text, false)
	}
	motion := "REDUCED"
	if m.settings.Animations {
		motion = "FULL"
	}
	drawText(screen, "T   NEXT THEME\nM   MOTION: "+motion+"\nESC EXIT", 950, screenH-90, 11, p.Muted, true)
}

const captureDuration = 1.15

func jelloScale(age float64) (x, y float64) {
	if age <= 0 {
		return .04, .04
	}
	if age >= captureDuration {
		return 1, 1
	}
	// The first third is a conspicuous center-out expansion. The remainder
	// holds near full size while alternating squash and stretch decay away.
	if age < .36 {
		t := age / .36
		base := .04 + 1.08*(1-math.Pow(1-t, 3))
		return base, base
	}
	t := (age - .36) / (captureDuration - .36)
	base := 1 + .08*(1-t)
	wobble := math.Sin(t*math.Pi*6) * (1 - t) * .16
	return base * (1 + wobble), base * (1 - wobble)
}

func (m *Match) drawCelebration(screen *ebiten.Image, p theme.Palette) {
	if m.gameOverAt.IsZero() {
		return
	}
	age := time.Since(m.gameOverAt).Seconds()
	if age < 0 {
		return
	}
	winners := m.state.Winners()
	if len(winners) == 0 {
		return
	}
	// Dim the playfield so the celebration reads as a distinct final scene
	// while leaving the completed board visible beneath it.
	maskAlpha := uint8(224 * math.Min(1, age/.35))
	vector.DrawFilledRect(screen, 0, 0, 920, screenH, color.RGBA{32, 34, 39, maskAlpha}, true)

	m.drawVillage(screen, p, winners, age)

	arrival := math.Min(1, age/.75)
	const c1 = 1.35
	eased := 1 + (c1+1)*math.Pow(arrival-1, 3) + c1*math.Pow(arrival-1, 2)
	bannerY := -120 + 170*eased
	shadow := color.RGBA{0, 0, 0, 90}
	vector.DrawFilledRect(screen, 157, float32(bannerY+8), 586, 108, shadow, true)
	banner := p.Panel
	banner.A = 248
	vector.DrawFilledRect(screen, 150, float32(bannerY), 586, 108, banner, true)
	vector.StrokeRect(screen, 150, float32(bannerY), 586, 108, 4, p.Accent, true)
	title := "VICTORY!"
	if len(winners) > 1 {
		title = "TIE GAME!"
	}
	drawCenteredText(screen, title, 443, bannerY+15, 32, p.Text, true)
	names := make([]string, len(winners))
	for i, winner := range winners {
		names[i] = m.settings.Players[winner].Name
	}
	drawCenteredText(screen, strings.Join(names, " & "), 443, bannerY+62, 17, p.Accent, true)
	if !m.settings.Animations {
		drawCenteredText(screen, "REDUCED MOTION  •  PRESS M FOR FULL CELEBRATION", 443, bannerY+126, 13, color.RGBA{235, 235, 235, 255}, true)
	}
}

const fireworkCycle = 3.1

var rainbow = []color.RGBA{
	{255, 49, 88, 255},  // red
	{255, 138, 50, 255}, // orange
	{255, 216, 77, 255}, // yellow
	{67, 224, 111, 255}, // green
	{49, 215, 232, 255}, // cyan
	{76, 125, 255, 255}, // blue
	{170, 92, 255, 255}, // violet
}

func drawCharacterFirework(screen *ebiten.Image, startX, startY, targetX, targetY, age float64, seed int) {
	event := int(math.Floor(age / fireworkCycle))
	colors := fireworkColors(seed, event)
	primary := colors[0]
	local := math.Mod(age, fireworkCycle)
	if local < .82 {
		t := local / .82
		eased := 1 - math.Pow(1-t, 2)
		x := startX + (targetX-startX)*eased
		y := startY + (targetY-startY)*eased
		// A layered comet tail makes the launch visibly originate at the
		// character's hand rather than simply appearing at its destination.
		for i := 11; i >= 0; i-- {
			trailT := math.Max(0, eased-float64(i)*.016)
			px := startX + (targetX-startX)*trailT
			py := startY + (targetY-startY)*trailT
			spark := primary
			spark.A = uint8(190 * (1 - float64(i)/12) * (1 - t*.25))
			size := float32(1.2 + (1-float64(i)/12)*1.8)
			vector.DrawFilledCircle(screen, float32(px), float32(py), size, spark, true)
		}
		glow := primary
		glow.A = 80
		vector.DrawFilledCircle(screen, float32(x), float32(y), 9, glow, true)
		vector.DrawFilledCircle(screen, float32(x), float32(y), 3.5, color.RGBA{255, 250, 225, 255}, true)
		return
	}
	t := local - .82
	const lifetime = 1.62
	if t > lifetime {
		return
	}
	fade := math.Pow(1-t/lifetime, .72)
	const streams = 48
	for stream := 0; stream < streams; stream++ {
		// A multicolor burst assigns one color to an entire radial stream.
		// Every particle along that ray therefore traces the same color all the
		// way out from the center instead of producing a mixed-color cloud.
		streamColor := colors[stream%len(colors)]
		angle := 2*math.Pi*float64(stream)/streams + unitHash(seed*101+event*37)*.12
		speed := 82 + unitHash(stream*43+seed*17)*48
		gravity := 68 + unitHash(stream*19+event)*34
		wind := (unitHash(stream*71+event*29) - .5) * 21
		for particle := 0; particle < 6; particle++ {
			particleAge := math.Max(0, t-float64(particle)*.026)
			spread := (float64(particle)-2.5)*.011 + (unitHash(stream*97+particle*31)-.5)*.018
			particleSpeed := speed * (.91 + unitHash(stream*131+particle*17)*.18)
			vx := math.Cos(angle+spread) * particleSpeed
			vy := math.Sin(angle+spread) * particleSpeed
			x := targetX + vx*particleAge + .5*wind*particleAge*particleAge
			y := targetY + vy*particleAge + .5*gravity*particleAge*particleAge
			particleColor := streamColor
			flicker := .35 + .65*math.Abs(math.Sin(age*31+float64(stream)*1.73+float64(particle)*.91))
			particleColor.A = uint8(240 * fade * flicker * (1 - float64(particle)*.09))
			size := math.Max(.65, (2.1-float64(particle)*.21)*(.75+.25*flicker))
			vector.DrawFilledCircle(screen, float32(x), float32(y), float32(size), particleColor, true)
		}
		if stream%3 == 0 {
			twinkle := streamColor
			flicker := .25 + .75*math.Abs(math.Sin(age*37+float64(stream)))
			twinkle.A = uint8(190 * fade * flicker)
			twinkleAge := math.Max(0, t-.07)
			twinkleSpeed := speed * (.55 + unitHash(stream+event*11)*.28)
			tx := targetX + math.Cos(angle+.045)*twinkleSpeed*twinkleAge + .5*wind*twinkleAge*twinkleAge
			ty := targetY + math.Sin(angle+.045)*twinkleSpeed*twinkleAge + .5*gravity*twinkleAge*twinkleAge
			vector.DrawFilledCircle(screen, float32(tx), float32(ty), float32(.8+flicker*.8), twinkle, true)
		}
	}
	// A short white ignition flash and expanding colored ring add depth while
	// preserving the fine particle detail around the center.
	if t < .22 {
		flash := 1 - t/.22
		vector.DrawFilledCircle(screen, float32(targetX), float32(targetY), float32(11*flash), color.RGBA{255, 252, 236, uint8(235 * flash)}, true)
		ring := primary
		ring.A = uint8(150 * flash)
		vector.StrokeCircle(screen, float32(targetX), float32(targetY), float32(8+t*95), 2, ring, true)
	}
}

func fireworkColors(seed, event int) []color.RGBA {
	signature := seed*17 + event*13
	count := 1
	if signature%7 == 0 {
		count = 2
	}
	if signature%19 == 0 {
		count = 3
	}
	base := (seed + event) % len(rainbow)
	if base < 0 {
		base += len(rainbow)
	}
	result := make([]color.RGBA, count)
	for i := range result {
		result[i] = rainbow[(base+i*2)%len(rainbow)]
	}
	return result
}

func unitHash(value int) float64 {
	x := math.Sin(float64(value)*12.9898+78.233) * 43758.5453
	return x - math.Floor(x)
}

func (m *Match) drawVillage(screen *ebiten.Image, p theme.Palette, winners []int, age float64) {
	signs := []string{"GO!", "DOT!", "GG!", "CHAMP!"}
	for i, character := range m.characters {
		row, column := i/5, i%5
		scale := .82
		baseX, baseline := 100+float64(column)*175, 650.
		if row == 1 {
			scale = .92
			baseX = 82 + float64(column)*177
			baseline = 790
		}
		x, jump, tilt, limbPhase := baseX, 0., 0., float64(i)*.73
		if m.settings.Animations {
			limbPhase = age*4.2 + float64(i)*.73
			x += math.Sin(age*.65+float64(i)*1.11) * 9
			jump = math.Pow(math.Max(0, math.Sin(limbPhase*.58)), 1.5) * (17 + float64(i%3)*4)
			tilt = math.Sin(limbPhase*.31) * .018
			if i%2 == 0 {
				fireworkAge := age + float64(i)*.4
				targetX := 95 + math.Mod(float64(i*193)+math.Floor(fireworkAge/fireworkCycle)*121, 710)
				targetY := 180 + float64((i*41)%170)
				drawCharacterFirework(screen, x+34, baseline-jump-105*scale, targetX, targetY, fireworkAge, i)
			}
		}
		hasSign := i%3 == 0
		signY := baseline - jump - 144*scale
		if hasSign {
			drawCrowdSign(screen, signs[(i/3)%len(signs)], x-82, signY, baseline-jump-57*scale, p.Players[winners[i%len(winners)]])
		}
		armSwing := math.Sin(limbPhase) * .19
		leftArm := -armSwing
		if hasSign {
			leftArm = -.23 + math.Sin(limbPhase)*.035
		}
		leftLeg := math.Sin(limbPhase+.7) * .14
		rightLeg := -math.Sin(limbPhase+.7) * .14
		headTurn := math.Sin(limbPhase*.31) * .025
		drawCharacterPart(screen, character.backHair, x, baseline-jump, scale, tilt, 60, 50, headTurn)
		drawCharacterPart(screen, character.leftLeg, x, baseline-jump, scale, tilt, 50, 107, leftLeg)
		drawCharacterPart(screen, character.rightLeg, x, baseline-jump, scale, tilt, 70, 107, rightLeg)
		drawCharacterPart(screen, character.leftArm, x, baseline-jump, scale, tilt, 38, 73, leftArm)
		drawCharacterPart(screen, character.rightArm, x, baseline-jump, scale, tilt, 82, 73, armSwing)
		drawCharacterPart(screen, character.body, x, baseline-jump, scale, tilt, 60, 80, 0)
		drawCharacterPart(screen, character.head, x, baseline-jump, scale, tilt, 60, 50, headTurn)
		drawCharacterPart(screen, character.prop, x, baseline-jump, scale, tilt, 82, 73, armSwing)
		if hasSign {
			poleX := x - 39
			grip := p.Players[winners[i%len(winners)]]
			vector.DrawFilledCircle(screen, float32(poleX), float32(baseline-jump-57*scale), 5, grip, true)
		}
	}
}

func drawCharacterPart(screen *ebiten.Image, part *ebiten.Image, x, baseline, scale, bodyTilt, jointX, jointY, rotation float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-jointX, -jointY)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(jointX, jointY)
	op.GeoM.Translate(-64, -140)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Rotate(bodyTilt)
	op.GeoM.Translate(x, baseline)
	screen.DrawImage(part, op)
}

func drawCrowdSign(screen *ebiten.Image, message string, x, y, holdY float64, accent color.RGBA) {
	shadow := color.RGBA{0, 0, 0, 90}
	vector.StrokeLine(screen, float32(x+43), float32(y+38), float32(x+43), float32(holdY), 7, shadow, true)
	vector.DrawFilledRect(screen, float32(x+4), float32(y+4), 86, 42, shadow, true)
	board := color.RGBA{244, 238, 218, 255}
	vector.DrawFilledRect(screen, float32(x), float32(y), 86, 42, board, true)
	vector.StrokeRect(screen, float32(x), float32(y), 86, 42, 4, accent, true)
	drawCenteredText(screen, message, x+43, y+9, 17, color.RGBA{35, 37, 43, 255}, true)
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
