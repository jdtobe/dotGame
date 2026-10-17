package display

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/jdtobe/dotGame/internal/config"
	"github.com/jdtobe/dotGame/internal/game"
	"github.com/jdtobe/dotGame/internal/theme"
)

const (
	sidebarX       = 920
	boardIntro     = .7  // dots pop in from the center when a match starts
	edgeReveal     = .18 // center-out draw of a newly claimed edge
	captureSparkle = .45
	capturePlusOne = .7 // floating +1 lifetime; the longest capture effect
	scorePop       = .4
	turnHandoff    = .3
	toastLife      = 2.6
)

// seatAvatars pairs seats with roster characters whose outfits echo the
// default seat colors.
var seatAvatars = [4]int{0, 3, 6, 4}

func (m *Match) Draw(screen *ebiten.Image) {
	initEffects()
	p := theme.ByID(m.settings.Theme)
	motion := m.settings.Animations
	m.drawBackdrop(screen, p)
	m.drawBoard(screen, p, motion)
	if m.state.Done() {
		m.drawCelebration(screen, p)
	}
	m.drawSidebar(screen, p, motion)
}

func since(t time.Time) float64 {
	if t.IsZero() {
		return math.Inf(1)
	}
	return time.Since(t).Seconds()
}

// glowBlend adds light on dark themes; additive light vanishes on light
// backgrounds, so those composite glows normally instead.
func glowBlend(p theme.Palette) ebiten.Blend {
	if isLight(p.Background) {
		return ebiten.BlendSourceOver
	}
	return ebiten.BlendLighter
}

func isLight(c color.RGBA) bool {
	return .2126*float64(c.R)+.7152*float64(c.G)+.0722*float64(c.B) > 150
}

// drawBackdrop lights the play area from behind the board and darkens the
// corners so attention settles on the grid.
func (m *Match) drawBackdrop(screen *ebiten.Image, p theme.Palette) {
	screen.Fill(p.Background)
	shade := .32
	if isLight(p.Background) {
		shade = .1
	}
	black := color.RGBA{0, 0, 0, 255}
	drawGradientQuad(screen, 0, 0, sidebarX, 150, black, shade, 0, ebiten.BlendSourceOver)
	drawGradientQuad(screen, 0, screenH-150, sidebarX, screenH, black, 0, shade, ebiten.BlendSourceOver)
	glow := mix(p.Background, p.Accent, .5)
	drawGlow(screen, 450, screenH/2, 560, 470, glow, .18, ebiten.BlendSourceOver)
}

// boardLayerKey identifies everything the cached board layer depends on.
type boardLayerKey struct {
	moves, unsettled int
	theme            string
	latestLive       bool
	introDone        bool
	motion           bool
}

// drawBoard composites a cached layer of the settled board with the parts
// that are animating this frame, so a full board stays cheap to draw.
func (m *Match) drawBoard(screen *ebiten.Image, p theme.Palette, motion bool) {
	left, top, step := m.geometry()
	intro := 1.
	if motion {
		intro = clamp01(since(m.startedAt) / boardIntro)
	}
	unsettled := 0
	if motion {
		for _, animation := range m.captureAnimations {
			if time.Since(animation.start).Seconds() < boxLiveFor {
				unsettled++
			}
		}
	}
	latestLive := motion && m.hasLastMove && since(m.lastMoveAt) < edgeLiveFor
	key := boardLayerKey{len(m.state.History), unsettled, p.ID, latestLive, intro >= 1, motion}
	if m.boardLayer == nil {
		m.boardLayer = ebiten.NewImage(sidebarX, screenH)
		m.boardLayerValid = false
	}
	if !m.boardLayerValid || key != m.boardLayerKey {
		m.boardLayer.Clear()
		m.drawBoardLayer(m.boardLayer, p, left, top, step, motion, latestLive, intro >= 1)
		m.boardLayerKey = key
		m.boardLayerValid = true
	}
	screen.DrawImage(m.boardLayer, nil)

	tx, ty, tw, th := m.trayRect(left, top, step)
	if !m.state.Done() {
		// The tray rim glows in the active seat's color and cross-fades on
		// each turn handoff.
		active := p.Players[m.state.Current]
		pulse, hand := 1., 1.
		if motion {
			pulse = .75 + .25*math.Sin(since(m.startedAt)*3)
			hand = easeOutCubic(clamp01(since(m.turnChangedAt) / turnHandoff))
		}
		strokeRoundedRect(screen, float32(tx-3), float32(ty-3), float32(tw+6), float32(th+6), 25, 3, withAlpha(active, .55*pulse*hand*intro))
		if hand < 1 {
			strokeRoundedRect(screen, float32(tx-3), float32(ty-3), float32(tw+6), float32(th+6), 25, 3, withAlpha(p.Players[m.previousTurn], .55*(1-hand)))
		}
	}
	edgeWidth := m.edgeWidth(step)
	if motion {
		m.drawBoxes(screen, p, left, top, step, motion, true)
		m.redrawLiveBoxFrames(screen, p, left, top, step, latestLive)
		m.drawPlusOnes(screen, p, left, top, step)
	}
	if latestLive {
		x1, y1, x2, y2 := movePoints(m.lastMove, left, top, step)
		m.drawClaimedEdge(screen, p, m.lastMove, x1, y1, x2, y2, edgeWidth, true)
	}
	if !m.state.Done() && m.settings.Players[m.state.Current].Kind == config.Human {
		x, y := ebiten.CursorPosition()
		if mv, ok := m.hitMove(float64(x), float64(y)); ok {
			m.drawHover(screen, p, mv, left, top, step, edgeWidth, motion)
		}
	}
	if intro < 1 {
		m.drawDots(screen, p, left, top, step, false)
	}
}

// redrawLiveBoxFrames puts the edges and corner dots of animating boxes back
// on top, so a box's overshoot never covers its own outline.
func (m *Match) redrawLiveBoxFrames(screen *ebiten.Image, p theme.Palette, left, top, step float64, skipLatest bool) {
	width := m.edgeWidth(step)
	radius := m.dotRadius(step)
	ring := withAlpha(trayColor(p), .9)
	for box, animation := range m.captureAnimations {
		if age := time.Since(animation.start).Seconds(); age < 0 || age >= captureDuration {
			continue
		}
		r, c := box/m.state.Width, box%m.state.Width
		for _, mv := range [4]game.Move{
			{Orientation: game.Horizontal, Row: r, Col: c},
			{Orientation: game.Horizontal, Row: r + 1, Col: c},
			{Orientation: game.Vertical, Row: r, Col: c},
			{Orientation: game.Vertical, Row: r, Col: c + 1},
		} {
			if skipLatest && m.hasLastMove && mv == m.lastMove {
				continue
			}
			x1, y1, x2, y2 := movePoints(mv, left, top, step)
			m.drawClaimedEdge(screen, p, mv, x1, y1, x2, y2, width, false)
		}
		for _, corner := range [4][2]int{{r, c}, {r, c + 1}, {r + 1, c}, {r + 1, c + 1}} {
			x, y := left+float64(corner[1])*step, top+float64(corner[0])*step
			vector.FillCircle(screen, float32(x), float32(y), float32(radius+1.6), ring, true)
			vector.FillCircle(screen, float32(x), float32(y), float32(radius), p.Dot, true)
		}
	}
}

func (m *Match) dotRadius(step float64) float64 { return math.Max(2.6, math.Min(4.6, step*.075)) }

const (
	boxLiveFor  = capturePlusOne // seconds a captured box animates before it is cached
	edgeLiveFor = .6             // seconds the newest edge animates before it is cached
)

func (m *Match) trayRect(left, top, step float64) (x, y, w, h float64) {
	pad := math.Max(22, step*.38)
	return left - pad, top - pad, step*float64(m.state.Width) + pad*2, step*float64(m.state.Height) + pad*2
}

func (m *Match) edgeWidth(step float64) float64 { return math.Max(4, math.Min(8, step*.1)) }

func trayColor(p theme.Palette) color.RGBA {
	if isLight(p.Background) {
		return mix(p.Panel, color.RGBA{255, 255, 255, 255}, .25)
	}
	return mix(p.Background, p.Panel, .85)
}

// drawBoardLayer renders everything that is not currently animating.
func (m *Match) drawBoardLayer(dst *ebiten.Image, p theme.Palette, left, top, step float64, motion, skipLatest, withDots bool) {
	tx, ty, tw, th := m.trayRect(left, top, step)
	drawGlow(dst, tx+tw/2, ty+th/2+26, tw*.62, th*.6, color.RGBA{0, 0, 0, 255}, .45, ebiten.BlendSourceOver)
	fillRoundedRect(dst, float32(tx), float32(ty), float32(tw), float32(th), 22, trayColor(p))
	strokeRoundedRect(dst, float32(tx), float32(ty), float32(tw), float32(th), 22, 1.5, withAlpha(p.Line, .9))
	m.drawBoxes(dst, p, left, top, step, motion, false)
	// Unclaimed edges first, so claimed ones and their glow sit above them.
	m.eachEdge(left, top, step, func(mv game.Move, x1, y1, x2, y2 float64) {
		if m.state.Owner(mv) < 0 {
			vector.StrokeLine(dst, float32(x1), float32(y1), float32(x2), float32(y2), 2, p.Line, true)
		}
	})
	width := m.edgeWidth(step)
	m.eachEdge(left, top, step, func(mv game.Move, x1, y1, x2, y2 float64) {
		if m.state.Owner(mv) < 0 || (skipLatest && m.hasLastMove && mv == m.lastMove) {
			return
		}
		m.drawClaimedEdge(dst, p, mv, x1, y1, x2, y2, width, false)
	})
	if withDots {
		m.drawDots(dst, p, left, top, step, true)
	}
}

func (m *Match) drawClaimedEdge(dst *ebiten.Image, p theme.Palette, mv game.Move, x1, y1, x2, y2, width float64, animate bool) {
	col := p.Players[m.state.Owner(mv)]
	age := since(m.lastMoveAt)
	if animate && age < edgeReveal {
		progress := easeOutCubic(age / edgeReveal)
		midX, midY := (x1+x2)/2, (y1+y2)/2
		dx, dy := (x2-x1)*progress/2, (y2-y1)*progress/2
		x1, y1, x2, y2 = midX-dx, midY-dy, midX+dx, midY+dy
	}
	glow := .16
	if isLight(p.Background) {
		glow = .1
	}
	strokeRound(dst, x1, y1, x2, y2, width*2.2, withAlpha(col, glow))
	strokeRound(dst, x1, y1, x2, y2, width, col)
	// A lighter core gives claimed edges a lit, tubular look.
	strokeRound(dst, x1, y1, x2, y2, width*.32, withAlpha(mix(col, color.RGBA{255, 255, 255, 255}, .55), .7))
	if animate && age < edgeLiveFor {
		f := 1 - age/edgeLiveFor
		strokeRound(dst, x1, y1, x2, y2, width+10*f, withAlpha(p.Hover, .45*f))
		for _, end := range [2][2]float64{{x1, y1}, {x2, y2}} {
			drawGlow(dst, end[0], end[1], 16*f+4, 16*f+4, col, .8*f, glowBlend(p))
		}
	}
}

// drawDots draws the dim intersection dots. When a match begins they pop in
// from the board center; settled draws them at full size.
func (m *Match) drawDots(dst *ebiten.Image, p theme.Palette, left, top, step float64, settled bool) {
	w, h := step*float64(m.state.Width), step*float64(m.state.Height)
	radius := m.dotRadius(step)
	cx, cy := left+w/2, top+h/2
	maxD := math.Hypot(w/2, h/2)
	ring := withAlpha(trayColor(p), .9)
	for r := 0; r <= m.state.Height; r++ {
		for c := 0; c <= m.state.Width; c++ {
			x, y := left+float64(c)*step, top+float64(r)*step
			scale := 1.
			if !settled {
				d := math.Hypot(x-cx, y-cy) / maxD
				local := clamp01((since(m.startedAt) - d*.35) / .35)
				if local <= 0 {
					continue
				}
				scale = easeOutBack(local, 2.2)
			}
			vector.FillCircle(dst, float32(x), float32(y), float32((radius+1.6)*scale), ring, true)
			vector.FillCircle(dst, float32(x), float32(y), float32(radius*scale), p.Dot, true)
		}
	}
}

func (m *Match) eachEdge(left, top, step float64, fn func(mv game.Move, x1, y1, x2, y2 float64)) {
	for r := 0; r <= m.state.Height; r++ {
		for c := 0; c < m.state.Width; c++ {
			fn(game.Move{Orientation: game.Horizontal, Row: r, Col: c}, left+float64(c)*step, top+float64(r)*step, left+float64(c+1)*step, top+float64(r)*step)
		}
	}
	for r := 0; r < m.state.Height; r++ {
		for c := 0; c <= m.state.Width; c++ {
			fn(game.Move{Orientation: game.Vertical, Row: r, Col: c}, left+float64(c)*step, top+float64(r)*step, left+float64(c)*step, top+float64(r+1)*step)
		}
	}
}

// drawBoxes renders claimed boxes beneath edges. A newly captured box pops in
// bright from half size, overshoots once, and cools to its resting tint while
// an impact ring and sparks fan out. live selects the boxes that are still
// animating; otherwise only settled boxes are drawn.
func (m *Match) drawBoxes(screen *ebiten.Image, p theme.Palette, left, top, step float64, motion, live bool) {
	monogramSize := math.Max(10, step*.36)
	for r := 0; r < m.state.Height; r++ {
		for c := 0; c < m.state.Width; c++ {
			owner := m.state.BoxOwner(r, c)
			if owner < 0 {
				continue
			}
			col := p.Players[owner]
			boxIndex := r*m.state.Width + c
			scale, fill, flash := 1., .24, 0.
			animation, animated := m.captureAnimations[boxIndex]
			age := math.Inf(1)
			if motion && animated {
				age = time.Since(animation.start).Seconds()
				if age < 0 {
					continue
				}
			}
			if live != (age < boxLiveFor) {
				continue
			}
			if age < captureDuration {
				scale = captureScale(age)
				// The fill lands bright and cools to its resting tint.
				settle := easeOutCubic(age / captureDuration)
				fill = .8 - .56*settle
				flash = .55 * (1 - clamp01(age/(capturePop*1.6)))
			}
			bx, by := left+(float64(c)+.5)*step, top+(float64(r)+.5)*step
			size := (step - 12) * scale
			radius := float32(size * .14)
			x0, y0 := float32(bx-size/2), float32(by-size/2)
			if age < captureDuration {
				// A thin ring expands past the box and fades: the impact.
				e := easeOutCubic(clamp01(age / (captureDuration * .85)))
				ring := (step - 12) * (1 + .3*e)
				strokeRoundedRect(screen, float32(bx-ring/2), float32(by-ring/2), float32(ring), float32(ring), float32(ring*.16), float32(3*(1-e)+1), withAlpha(col, .85*(1-e)))
			}
			fillRoundedRect(screen, x0, y0, float32(size), float32(size), radius, withAlpha(col, fill))
			if flash > 0 {
				fillRoundedRect(screen, x0, y0, float32(size), float32(size), radius, withAlpha(mix(col, color.RGBA{255, 255, 255, 255}, .7), flash))
			}
			// Inner sheen across the upper half.
			fillRoundedRect(screen, x0+3, y0+3, float32(size-6), float32(size*.42), radius, withAlpha(color.RGBA{255, 255, 255, 255}, .06))
			// The owner's initial keeps territory legible without relying on hue.
			initial := strings.ToUpper(string([]rune(m.settings.Players[owner].Name)[:1]))
			face := &text.GoTextFace{Source: boldFont, Size: monogramSize}
			drawTextCentered(screen, initial, face, bx, by, scale, 0, col, .55)
			if age < captureSparkle {
				t := age / captureSparkle
				for k := 0; k < 8; k++ {
					a := float64(k)*math.Pi/4 + float64(boxIndex)
					d := step * (.3 + .45*easeOutCubic(t))
					drawGlow(screen, bx+math.Cos(a)*d, by+math.Sin(a)*d, 6*(1-t)+2, 6*(1-t)+2, col, 1-t, glowBlend(p))
				}
				drawGlow(screen, bx, by, step*.75, step*.75, col, .45*(1-t)*(1-t), glowBlend(p))
			}
		}
	}
}

// drawPlusOnes floats a +1 above each newly captured box, above its outline.
func (m *Match) drawPlusOnes(screen *ebiten.Image, p theme.Palette, left, top, step float64) {
	plus := &text.GoTextFace{Source: boldFont, Size: math.Max(14, step*.3)}
	for box, animation := range m.captureAnimations {
		age := time.Since(animation.start).Seconds()
		if age < 0 || age >= capturePlusOne {
			continue
		}
		col := p.Players[m.state.Boxes[box]]
		bx := left + (float64(box%m.state.Width)+.5)*step
		by := top + (float64(box/m.state.Width)+.5)*step
		t := age / capturePlusOne
		lift := easeOutCubic(t) * step * .5
		alpha := clamp01(t/.08) * (1 - smoothstep(clamp01((t-.5)/.5)))
		drawTextCentered(screen, "+1", plus, bx+1.5, by-step*.42-lift+2, 1, 0, color.RGBA{0, 0, 0, 255}, alpha*.5)
		drawTextCentered(screen, "+1", plus, bx, by-step*.42-lift, 1, 0, mix(col, color.RGBA{255, 255, 255, 255}, .35), alpha)
	}
}

// drawHover previews the edge under the cursor and outlines any box the move
// would complete.
func (m *Match) drawHover(screen *ebiten.Image, p theme.Palette, mv game.Move, left, top, step, width float64, motion bool) {
	x1, y1, x2, y2 := movePoints(mv, left, top, step)
	pulse := 1.
	if motion {
		pulse = .8 + .2*math.Sin(since(m.startedAt)*7)
	}
	strokeRound(screen, x1, y1, x2, y2, width+8, withAlpha(p.Hover, .18*pulse))
	strokeRound(screen, x1, y1, x2, y2, width, withAlpha(p.Hover, .85*pulse))
	for _, end := range [2][2]float64{{x1, y1}, {x2, y2}} {
		vector.FillCircle(screen, float32(end[0]), float32(end[1]), float32(width*.9), p.Hover, true)
	}
	active := p.Players[m.state.Current]
	preview := func(r, c int) {
		if r < 0 || c < 0 || r >= m.state.Height || c >= m.state.Width || m.state.BoxSides(r, c) != 3 {
			return
		}
		bx, by := left+float64(c)*step+8, top+float64(r)*step+8
		fillRoundedRect(screen, float32(bx), float32(by), float32(step-16), float32(step-16), float32(step*.12), withAlpha(active, .16*pulse))
		strokeRoundedRect(screen, float32(bx), float32(by), float32(step-16), float32(step-16), float32(step*.12), 2, withAlpha(active, .7*pulse))
	}
	if mv.Orientation == game.Horizontal {
		preview(mv.Row-1, mv.Col)
		preview(mv.Row, mv.Col)
	} else {
		preview(mv.Row, mv.Col-1)
		preview(mv.Row, mv.Col)
	}
}

func strokeRound(dst *ebiten.Image, x1, y1, x2, y2, width float64, clr color.RGBA) {
	var path vector.Path
	path.MoveTo(float32(x1), float32(y1))
	path.LineTo(float32(x2), float32(y2))
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.StrokePath(dst, &path, &vector.StrokeOptions{Width: float32(width), LineCap: vector.LineCapRound}, op)
}

func (m *Match) drawSidebar(screen *ebiten.Image, p theme.Palette, motion bool) {
	vector.FillRect(screen, sidebarX, 0, screenW-sidebarX, screenH, p.Panel, false)
	vector.FillRect(screen, sidebarX, 0, 1, screenH, p.Line, false)
	drawGlow(screen, sidebarX+140, 0, 220, 130, p.Accent, .1, ebiten.BlendSourceOver)
	const x = sidebarX + 26
	const cardW = screenW - sidebarX - 52

	// Wordmark: a tiny claimed box between four dots.
	vector.FillRect(screen, x+4, 37, 18, 18, withAlpha(p.Accent, .3), true)
	for _, d := range [4][2]float32{{x + 2, 35}, {x + 24, 35}, {x + 2, 57}, {x + 24, 57}} {
		vector.FillCircle(screen, d[0], d[1], 3, p.Text, true)
	}
	vector.StrokeLine(screen, x+2, 35, x+24, 35, 3, p.Accent, true)
	drawText(screen, "DOTGAME", x+40, 30, 26, p.Text, true)
	chipFace := &text.GoTextFace{Source: boldFont, Size: 10}
	themeName := strings.ToUpper(p.Name)
	cw, _ := text.Measure(themeName, chipFace, 0)
	strokeRoundedRect(screen, x, 76, float32(cw+20), 20, 10, 1.2, withAlpha(p.Accent, .8))
	drawTextCentered(screen, themeName, chipFace, x+(cw+20)/2, 86, 1, 0, p.Accent, 1)

	// Territory bar: each seat's share of the whole board.
	total := m.state.Width * m.state.Height
	claimed := 0
	for _, s := range m.state.Scores {
		claimed += s
	}
	small := &text.GoTextFace{Source: boldFont, Size: 10}
	drawText(screen, fmt.Sprintf("%d × %d BOARD", m.state.Width, m.state.Height), x, 112, 10, p.Muted, true)
	label := fmt.Sprintf("%d / %d", claimed, total)
	lw, _ := text.Measure(label, small, 0)
	drawText(screen, label, x+cardW-lw, 112, 10, p.Muted, true)
	fillRoundedRect(screen, x, 130, cardW, 8, 4, withAlpha(p.Line, .8))
	bx := float64(x)
	for i, s := range m.state.Scores {
		seg := float64(cardW) * float64(s) / float64(total)
		if seg > 0 {
			vector.FillRect(screen, float32(bx), 130, float32(seg), 8, p.Players[i], true)
		}
		bx += seg
	}

	y := 158.
	cardH := 82.
	if len(m.settings.Players) > 3 {
		cardH = 76
	}
	for i, pl := range m.settings.Players {
		m.drawPlayerCard(screen, p, i, pl, x, y, cardW, cardH, motion)
		y += cardH + 10
	}

	if m.status != "" {
		age := since(m.statusAt)
		alpha := 1.
		if motion {
			alpha = clamp01(age/.15) * (1 - smoothstep(clamp01((age-toastLife+.5)/.5)))
		} else if age > toastLife {
			alpha = 0
		}
		if alpha > 0 {
			rise := 0.
			if motion {
				rise = (1 - easeOutCubic(clamp01(age/.25))) * 8
			}
			face := &text.GoTextFace{Source: regularFont, Size: 12}
			fillRoundedRect(screen, x, float32(y+6+rise), cardW, 32, 16, withAlpha(mix(p.Panel, p.Text, .08), alpha))
			vector.FillCircle(screen, x+16, float32(y+22+rise), 4, withAlpha(p.Accent, alpha), true)
			drawTextCentered(screen, m.status, face, float64(x)+28+measure(m.status, face)/2, y+22+rise, 1, 0, p.Text, alpha)
		}
	}

	if m.state.Done() {
		w := m.state.Winners()
		names := make([]string, len(w))
		for i, id := range w {
			names[i] = m.settings.Players[id].Name
		}
		gy := float32(screenH - 230)
		fillRoundedRect(screen, x, gy, cardW, 64, 14, withAlpha(p.Accent, .14))
		strokeRoundedRect(screen, x, gy, cardW, 64, 14, 1.5, withAlpha(p.Accent, .7))
		drawText(screen, "GAME COMPLETE", x+16, float64(gy)+12, 13, p.Accent, true)
		drawText(screen, "Winner: "+strings.Join(names, " & "), x+16, float64(gy)+34, 13, p.Text, false)
	}

	motionLabel := "MOTION: REDUCED"
	if m.settings.Animations {
		motionLabel = "MOTION: FULL"
	}
	keys := [][2]string{{"T", "NEXT THEME"}, {"M", motionLabel}, {"ESC", "EXIT"}}
	if m.state.Done() {
		keys = append([][2]string{{"ENTER", "PLAY AGAIN"}}, keys...)
	}
	ky := float64(screenH - 24 - 26*len(keys))
	keyFace := &text.GoTextFace{Source: boldFont, Size: 10}
	for _, k := range keys {
		kw := math.Max(22, measure(k[0], keyFace)+12)
		fillRoundedRect(screen, x, float32(ky), float32(kw), 20, 5, withAlpha(p.Line, .9))
		vector.FillRect(screen, x+2, float32(ky+17), float32(kw-4), 2, withAlpha(color.RGBA{0, 0, 0, 255}, .2), false)
		drawTextCentered(screen, k[0], keyFace, float64(x)+kw/2, ky+9.5, 1, 0, p.Text, 1)
		drawText(screen, k[1], float64(x)+kw+10, ky+3, 10, p.Muted, true)
		ky += 26
	}
}

func measure(value string, face *text.GoTextFace) float64 {
	w, _ := text.Measure(value, face, 0)
	return w
}

func (m *Match) drawPlayerCard(screen *ebiten.Image, p theme.Palette, i int, pl config.Player, x, y, w, h float64, motion bool) {
	col := p.Players[i]
	done := m.state.Done()
	// Activation eases between seats so the turn visibly hands off.
	active := 0.
	hand := 1.
	if motion {
		hand = easeOutCubic(clamp01(since(m.turnChangedAt) / turnHandoff))
	}
	if !done && i == m.state.Current {
		active = hand
	} else if !done && i == m.previousTurn && hand < 1 {
		active = 1 - hand
	}
	isWinner := false
	if done {
		for _, wi := range m.state.Winners() {
			isWinner = isWinner || wi == i
		}
		if isWinner {
			active = 1
		}
	}
	card := mix(p.Panel, p.Background, .45)
	fillRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, card)
	if active > 0 {
		fillRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, withAlpha(col, .14*active))
		strokeRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, 2, withAlpha(col, active))
		drawGlow(screen, x+w/2, y+h/2, w*.6, h*.8, col, .12*active, glowBlend(p))
	} else {
		strokeRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, 1, withAlpha(p.Line, .9))
	}

	// Portrait: the seat's roster character, framed in the seat color.
	ax, ay := x+12, y+h/2-26
	fillRoundedRect(screen, float32(ax), float32(ay), 52, 52, 12, withAlpha(col, .28))
	strokeRoundedRect(screen, float32(ax), float32(ay), 52, 52, 12, 2, col)
	if len(m.characters) > 0 {
		character := m.characters[seatAvatars[i%len(seatAvatars)]%len(m.characters)]
		bob := 0.
		if motion && active > .5 && !done {
			bob = math.Sin(since(m.startedAt)*5) * 1.5 * active
		}
		pose := rigPose{x: ax + 26, ground: ay + 82 + bob, scale: .56}
		if isWinner && motion {
			pose.tilt = math.Sin(since(m.startedAt)*4) * .08
		}
		drawRigPart(screen, character.backHair, pose, 60, 50, 0)
		drawRigPart(screen, character.head, pose, 60, 50, 0)
	}

	tx := x + 76
	name := strings.ToUpper(pl.Name)
	drawText(screen, name, tx, y+h/2-22, 14, p.Text, true)
	kind := "HUMAN"
	if pl.Kind == config.AI {
		kind = "AI · " + strings.ToUpper(pl.Difficulty)
	}
	tag := kind
	tagColor := p.Muted
	switch {
	case done && isWinner:
		tag, tagColor = "WINNER", col
	case active > .5 && pl.Kind == config.AI && m.aiThinking:
		dots := 3
		if motion {
			dots = 1 + int(since(m.startedAt)*3)%3
		}
		tag, tagColor = "THINKING"+strings.Repeat(".", dots), col
	case active > .5:
		tag, tagColor = "YOUR TURN", col
		if pl.Kind == config.AI {
			tag = "TURN"
		}
	}
	if done && isWinner {
		drawCrown(screen, tx+7, y+h/2+9, 6.5, color.RGBA{255, 206, 74, 255}, 1)
		tx += 19
	}
	drawText(screen, tag, tx, y+h/2+2, 10, tagColor, true)

	// Score, with a pop each time a captured box lands.
	score := fmt.Sprintf("%d", m.state.Scores[i])
	scale := 1.
	scoreColor := p.Text
	if motion {
		if t := since(m.scoreBumpAt[i]) - capturePop; t >= 0 && t < scorePop {
			k := t / scorePop
			scale = 1 + .45*math.Sin(math.Pi*k)*(1-k)
			scoreColor = mix(col, p.Text, k)
		}
	}
	face := &text.GoTextFace{Source: boldFont, Size: 28}
	sw := measure(score, face)
	drawTextCentered(screen, score, face, x+w-16-sw/2, y+h/2-2, scale, 0, scoreColor, 1)
}
