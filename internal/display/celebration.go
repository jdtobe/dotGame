package display

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/jdtobe/dotGame/internal/config"
	"github.com/jdtobe/dotGame/internal/theme"
)

// The celebration is a fixed timeline measured from gameOverAt. Every element
// is a pure function of elapsed time, so reduced motion simply evaluates the
// settled end state with the looping clock held at zero.
const (
	celebrationBeat    = 60.0 / 124 // crowd choreography tempo
	bannerArrive       = .12
	titleSlamStart     = .4
	titleLetterStagger = .07
	crowdEnterStart    = .55
	crowdEnterDuration = .62
	confettiBlast      = .42
	promptAppear       = 2.1
	fireworkCycle      = 3.1
	fireworkAscent     = .75
	fireworkLifetime   = 1.9

	stageW     = 920 // the board area; the score panel stays visible
	stageCX    = 450
	bannerCY   = 172
	bannerW    = 800
	bannerH    = 330
	panelInset = 110 // panel x offset inside the banner canvas
	panelTop   = 70
	panelW     = 580
	panelH     = 200
)

var (
	effectsOnce  sync.Once
	glowImage    *ebiten.Image // soft radial falloff for light and particles
	whitePixel   *ebiten.Image // solid source for vertex-colored triangles
	bannerCanvas *ebiten.Image // offscreen title card, scaled as one unit
	signImages   = map[string]*ebiten.Image{}
)

func initEffects() {
	effectsOnce.Do(func() {
		const size = 64
		img := image.NewRGBA(image.Rect(0, 0, size, size))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				dx := (float64(x) + .5 - size/2) / (size / 2)
				dy := (float64(y) + .5 - size/2) / (size / 2)
				d := math.Min(1, math.Hypot(dx, dy))
				a := uint8(255 * math.Pow(1-d, 2.2))
				img.Pix[y*img.Stride+x*4+0] = a
				img.Pix[y*img.Stride+x*4+1] = a
				img.Pix[y*img.Stride+x*4+2] = a
				img.Pix[y*img.Stride+x*4+3] = a
			}
		}
		glowImage = ebiten.NewImageFromImage(img)
		base := ebiten.NewImage(3, 3)
		base.Fill(color.White)
		whitePixel = base.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
		bannerCanvas = ebiten.NewImage(bannerW, bannerH)
	})
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
	initEffects()
	motion := m.settings.Animations
	// intro drives one-shot entrances; loop drives continuous motion.
	intro, loop := age, age
	if !motion {
		intro, loop = 60, 0
	}
	accents := make([]color.RGBA, len(winners))
	for i, w := range winners {
		accents[i] = p.Players[w]
	}
	m.drawStage(screen, p, winners, accents, intro, loop, motion)
	poses := m.crowdPoses(intro, loop, motion)
	if motion {
		drawFireworks(screen, poses, intro)
	}
	m.drawCrowd(screen, p, winners, accents, poses, loop)
	drawConfetti(screen, accents, intro, motion)
	m.drawBanner(screen, p, winners, accents, intro, loop, motion)
	drawPrompt(screen, p, intro, loop, motion)
	if motion && intro < .3 {
		// A brief exposure flash marks the instant the match is decided.
		flash := 1 - intro/.3
		drawGradientQuad(screen, 0, 0, stageW, screenH, color.RGBA{255, 255, 255, 255}, .24*flash, .24*flash, ebiten.BlendLighter)
	}
}

// drawStage turns the board into a night-time arena: a theme-tinted backdrop,
// a light wave across the winner's territory, sweeping spotlights, a lit
// floor, and a sunburst behind the title card.
func (m *Match) drawStage(screen *ebiten.Image, p theme.Palette, winners []int, accents []color.RGBA, intro, loop float64, motion bool) {
	fade := smoothstep(clamp01(intro / .45))
	night := mix(mix(p.Background, color.RGBA{6, 7, 14, 255}, .8), accents[0], .07)
	vector.FillRect(screen, 0, 0, stageW, screenH, withAlpha(night, .93*fade), false)

	left, top, step := m.geometry()
	centerX := left + step*float64(m.state.Width)/2
	centerY := top + step*float64(m.state.Height)/2
	maxDistance := math.Hypot(step*float64(m.state.Width)/2, step*float64(m.state.Height)/2)
	isWinner := map[int]bool{}
	for _, w := range winners {
		isWinner[w] = true
	}
	for r := 0; r < m.state.Height; r++ {
		for c := 0; c < m.state.Width; c++ {
			owner := m.state.BoxOwner(r, c)
			if !isWinner[owner] {
				continue
			}
			bx, by := left+(float64(c)+.5)*step, top+(float64(r)+.5)*step
			d := math.Hypot(bx-centerX, by-centerY) / maxDistance
			pulse := 0.
			if motion {
				// One bright shockwave from the center, then a gentle
				// recurring ripple keeps the claimed territory alive.
				first := intro - .12 - d*.6
				pulse = math.Exp(-first * first / .02)
				if loop > 1.6 {
					ripple := math.Mod(loop-1.6, 4.4) - d*.9
					pulse += .4 * math.Exp(-ripple*ripple/.03)
				}
			}
			col := p.Players[owner]
			size := (step - 10) * (1 + .1*pulse)
			vector.FillRect(screen, float32(bx-size/2), float32(by-size/2), float32(size), float32(size), withAlpha(col, (.2+.6*pulse)*fade), true)
			if pulse > .04 {
				drawGlow(screen, bx, by, step*.95, step*.95, col, .45*pulse, ebiten.BlendLighter)
			}
		}
	}

	// A lit floor grounds the crowd instead of leaving it floating on the board.
	floor := mix(accents[0], color.RGBA{0, 0, 0, 255}, .6)
	drawGradientQuad(screen, 0, 545, stageW, screenH, floor, 0, .7*fade, ebiten.BlendSourceOver)
	drawGlow(screen, stageCX, 835, 560, 175, accents[0], .3*fade, ebiten.BlendLighter)
	drawGlow(screen, stageCX, 690, 520, 80, accents[len(accents)-1], .12*fade, ebiten.BlendLighter)

	for k := 0; k < 2; k++ {
		apexX := 30. + float64(k)*860
		aimX := stageCX + math.Sin(loop*.55+float64(k)*2.4)*250
		if !motion {
			aimX = stageCX + (float64(k)*2-1)*160
		}
		light := mix(color.RGBA{255, 255, 255, 255}, accents[k%len(accents)], .45)
		drawBeam(screen, apexX, -40, aimX, 830, 110, light, .11*fade)
		drawGlow(screen, aimX, 805, 170, 36, light, .24*fade, ebiten.BlendLighter)
	}

	grow := easeOutCubic(clamp01((intro - .2) / .9))
	drawSunburst(screen, stageCX, bannerCY, 470*grow, loop*.1, accents, .09*fade)
	drawGlow(screen, stageCX, bannerCY, 380*grow, 230*grow, accents[0], .22*fade, ebiten.BlendLighter)
}

type danceRole uint8

const (
	roleHop danceRole = iota
	rolePump
	roleWave
	roleSign
	roleRocket
)

// crowdRoles assigns choreography by slot: signs and rockets sit in the back
// row where they never cover a face; the front row jumps and cheers.
var crowdRoles = [10]danceRole{roleSign, roleRocket, roleWave, roleRocket, roleSign, rolePump, roleHop, roleRocket, roleHop, rolePump}

// rigPose is one character's full articulation for a frame.
type rigPose struct {
	x, ground, jump    float64
	scale, squash      float64
	tilt, head         float64
	armL, armR         float64
	legL, legR         float64
	visible            bool
	role               danceRole
	launcher, rocketID int
}

func crowdSlot(i int) (x, ground, scale float64) {
	if i < 5 {
		return 120 + float64(i)*165, 652, .9
	}
	return 72 + float64(i-5)*189, 808, 1.1
}

func crowdDelay(i int) float64 {
	x, _, _ := crowdSlot(i)
	delay := crowdEnterStart + math.Abs(x-stageCX)/stageCX*.35
	if i < 5 {
		delay += .12
	}
	return delay
}

// rocketLaunchers are crowd slots that fire rockets, followed by two unseen
// launchers at the stage edges so the sky never goes quiet.
var rocketLaunchers = []int{1, 3, 7, -1, -2}

func launchStart(launcher int) float64 { return 1.05 + float64(launcher)*.62 }

func (m *Match) crowdPoses(intro, loop float64, motion bool) []rigPose {
	poses := make([]rigPose, len(m.characters))
	for i := range poses {
		x, ground, scale := crowdSlot(i)
		pose := rigPose{x: x, ground: ground, scale: scale, role: crowdRoles[i%len(crowdRoles)], launcher: -1}
		for j, slot := range rocketLaunchers {
			if slot == i {
				pose.launcher = j
			}
		}
		delay := crowdDelay(i)
		enter := clamp01((intro - delay) / crowdEnterDuration)
		if enter <= 0 {
			poses[i] = pose
			continue
		}
		pose.visible = true
		beat := loop / celebrationBeat
		if !motion {
			beat = .5
		}
		dance(&pose, beat, i, intro)
		if !motion {
			pose.jump, pose.squash, pose.tilt = 0, 0, 0
		}
		// Characters spring up from below the stage and land with a squash.
		pose.jump -= (1 - easeOutBack(enter, 1.7)) * 240
		landing := intro - delay - crowdEnterDuration
		if motion && landing > -.05 {
			pose.squash += .16 * math.Exp(-landing*landing/.006)
		}
		poses[i] = pose
	}
	return poses
}

// dance poses a character for the beat. Arm angles follow the rig: a positive
// left-arm rotation raises it, a negative right-arm rotation lifts it overhead.
func dance(p *rigPose, beat float64, i int, intro float64) {
	switch p.role {
	case roleHop:
		phase := frac((beat + float64(i%2)) / 2)
		if phase < .62 {
			a := phase / .62
			h := 4 * a * (1 - a)
			p.jump = 36 * h
			p.squash = -.07 * h
			p.armL, p.armR = .35+1.05*h, -.05-.5*h
			p.legL, p.legR = .26*h, -.26*h
			p.head = -.06 * h
		} else {
			g := (phase - .62) / .38
			p.squash = .14 * math.Sin(math.Pi*g)
			p.armL, p.armR = .3, .25
			p.head = .04 * math.Sin(math.Pi*g)
		}
	case rolePump:
		swing := math.Sin(math.Pi * beat)
		p.jump = 7 * math.Abs(swing)
		p.squash = .05 * math.Cos(2*math.Pi*beat)
		p.armR = .15 - .7*math.Max(0, swing)
		p.armL = .25 + 1.15*math.Max(0, -swing)
		p.legL, p.legR = .08*swing, .08*swing
		p.head = .05 * swing
	case roleWave:
		p.tilt = .06 * math.Sin(math.Pi*beat/2)
		p.jump = 4 * math.Abs(math.Sin(math.Pi*beat))
		p.armR = -.28 + .4*math.Sin(2*math.Pi*beat)
		p.armL = .2 + .12*math.Sin(math.Pi*beat)
		p.legL, p.legR = -p.tilt, -p.tilt
		p.head = -p.tilt * .8
	case roleSign:
		p.armL = 1.05 + .06*math.Sin(math.Pi*beat)
		p.armR = -.12 + .3*math.Sin(math.Pi*beat)
		p.jump = 5 * math.Abs(math.Sin(math.Pi*beat))
		p.tilt = .035 * math.Sin(math.Pi*beat/2)
		p.head = .05 * math.Sin(math.Pi*beat/2)
	case roleRocket:
		p.jump = 4 * math.Abs(math.Sin(math.Pi*beat))
		p.armL = .3 + .4*math.Max(0, math.Sin(math.Pi*beat))
		thrust := 0.
		if p.launcher >= 0 {
			// The throwing arm winds up just before each launch and points
			// skyward as the rocket leaves the hand.
			u := math.Mod(intro-launchStart(p.launcher)+fireworkCycle*4, fireworkCycle)
			if u > fireworkCycle-.3 {
				u -= fireworkCycle
			}
			thrust = math.Exp(-(u - .05) * (u - .05) / .025)
		}
		p.armR = .25 - .8*thrust + .08*math.Sin(math.Pi*beat)
		p.squash = .08*thrust - .04
		p.head = -.07 * thrust
	}
}

// rigGeoM maps layer units (128×140) into screen space for one jointed part.
func (p rigPose) rigGeoM(jointX, jointY, rotation float64) ebiten.GeoM {
	var g ebiten.GeoM
	g.Translate(-jointX, -jointY)
	g.Rotate(rotation)
	g.Translate(jointX, jointY)
	g.Translate(-layerWidth/2, -layerHeight)
	g.Scale(p.scale*(1+p.squash), p.scale*(1-p.squash))
	g.Rotate(p.tilt)
	g.Translate(p.x, p.ground-p.jump)
	return g
}

// Shoulder pivots and hand centers in layer units, derived from the source
// viewBox at 0.4 scale.
const (
	leftShoulderX, leftShoulderY   = 38, 73
	rightShoulderX, rightShoulderY = 82, 73
	leftHandX, leftHandY           = 14.8, 75.2
	rightHandX, rightHandY         = 100, 50.8
)

func (p rigPose) leftHand() (float64, float64) {
	g := p.rigGeoM(leftShoulderX, leftShoulderY, p.armL)
	return g.Apply(leftHandX, leftHandY)
}

func (p rigPose) rightHand() (float64, float64) {
	g := p.rigGeoM(rightShoulderX, rightShoulderY, p.armR)
	return g.Apply(rightHandX, rightHandY)
}

func drawRigPart(screen, part *ebiten.Image, p rigPose, jointX, jointY, rotation float64) {
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(1./layerSupersample, 1./layerSupersample)
	op.GeoM.Concat(p.rigGeoM(jointX, jointY, rotation))
	screen.DrawImage(part, op)
}

func (m *Match) drawCrowd(screen *ebiten.Image, p theme.Palette, winners []int, accents []color.RGBA, poses []rigPose, loop float64) {
	signs := m.signMessages(winners)
	for i, pose := range poses {
		if !pose.visible {
			continue
		}
		// Contact shadows shrink and fade as a character leaves the ground.
		lift := clamp01(math.Max(0, pose.jump) / 90)
		drawGlow(screen, pose.x, pose.ground+2, 50*pose.scale*(1-.35*lift), 11*pose.scale*(1-.3*lift), color.RGBA{0, 0, 0, 255}, .6*(1-.6*lift), ebiten.BlendSourceOver)
		if pose.role == roleSign {
			hx, hy := pose.leftHand()
			sway := pose.tilt + .07*math.Sin(loop*2.3+float64(i))
			drawHeldSign(screen, signs[(i/2)%len(signs)], hx, hy, pose.scale, sway, accents[(i/2)%len(accents)])
		}
		c := m.characters[i]
		drawRigPart(screen, c.backHair, pose, 60, 50, pose.head)
		drawRigPart(screen, c.leftLeg, pose, 50, 107, pose.legL)
		drawRigPart(screen, c.rightLeg, pose, 70, 107, pose.legR)
		drawRigPart(screen, c.leftArm, pose, leftShoulderX, leftShoulderY, pose.armL)
		drawRigPart(screen, c.rightArm, pose, rightShoulderX, rightShoulderY, pose.armR)
		drawRigPart(screen, c.body, pose, 60, 80, 0)
		drawRigPart(screen, c.head, pose, 60, 50, pose.head)
		drawRigPart(screen, c.prop, pose, rightShoulderX, rightShoulderY, pose.armR)
	}
}

func (m *Match) signMessages(winners []int) []string {
	if len(winners) > 1 {
		return []string{"GG!", "TIE!", "WOW!"}
	}
	name := strings.ToUpper(m.settings.Players[winners[0]].Name)
	if len([]rune(name)) > 7 {
		name = string([]rune(name)[:7])
	}
	return []string{"GG!", name + "!", "CHAMP!"}
}

// drawHeldSign draws a placard on a pole whose grip sits in the given hand.
// It is drawn before the character so the pole passes behind the fist.
func drawHeldSign(screen *ebiten.Image, message string, handX, handY, scale, angle float64, accent color.RGBA) {
	sign := signImage(message, accent)
	pole := 70 * scale
	topX := handX + math.Sin(angle)*pole
	topY := handY - math.Cos(angle)*pole
	vector.StrokeLine(screen, float32(handX), float32(handY+10*scale), float32(topX), float32(topY), float32(8*scale), color.RGBA{28, 22, 30, 255}, true)
	vector.StrokeLine(screen, float32(handX), float32(handY+10*scale), float32(topX), float32(topY), float32(4.5*scale), color.RGBA{196, 142, 92, 255}, true)
	w, h := float64(sign.Bounds().Dx()), float64(sign.Bounds().Dy())
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-w/2, -h+6)
	op.GeoM.Scale(scale/layerSupersample, scale/layerSupersample)
	op.GeoM.Rotate(angle)
	op.GeoM.Translate(topX, topY)
	screen.DrawImage(sign, op)
}

func signImage(message string, accent color.RGBA) *ebiten.Image {
	key := fmt.Sprintf("%s#%02x%02x%02x", message, accent.R, accent.G, accent.B)
	if img, ok := signImages[key]; ok {
		return img
	}
	const s = layerSupersample
	const w, h = 108, 52
	img := ebiten.NewImage(w*s, (h+6)*s)
	fillRoundedRect(img, 4*s, 6*s, (w-4)*s, (h-2)*s, 10*s, color.RGBA{0, 0, 0, 90})
	fillRoundedRect(img, 0, 0, (w-4)*s, (h-4)*s, 10*s, color.RGBA{28, 22, 30, 255})
	fillRoundedRect(img, 4*s, 4*s, (w-12)*s, (h-12)*s, 7*s, color.RGBA{255, 249, 234, 255})
	vector.FillRect(img, 4*s, float32(h-14)*s, (w-12)*s, 6*s, accent, true)
	size := 21.
	face := &text.GoTextFace{Source: boldFont, Size: size * s}
	for tw, _ := text.Measure(message, face, 0); tw > (w-24)*s && size > 10; tw, _ = text.Measure(message, face, 0) {
		size--
		face.Size = size * s
	}
	tw, th := text.Measure(message, face, 0)
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64((w-4)*s)/2-tw/2, float64((h-10)*s)/2-th/2)
	op.ColorScale.ScaleWithColor(color.RGBA{34, 30, 44, 255})
	text.Draw(img, message, face, op)
	signImages[key] = img
	return img
}

// fireworkSlots keep bursts in open sky around, not behind, the title card.
var fireworkSlots = [][2]float64{{110, 165}, {790, 160}, {240, 385}, {665, 375}, {450, 430}, {150, 330}, {760, 320}, {340, 445}, {565, 445}}

func drawFireworks(screen *ebiten.Image, poses []rigPose, intro float64) {
	for j, slot := range rocketLaunchers {
		local := intro - launchStart(j)
		if local < 0 {
			continue
		}
		var startX, startY float64
		switch {
		case slot >= 0 && slot < len(poses):
			startX, startY = poses[slot].rightHand()
		case slot == -1:
			startX, startY = 20, screenH+20
		default:
			startX, startY = stageW-20, screenH+20
		}
		event := int(local / fireworkCycle)
		target := fireworkSlots[(j*4+event*3)%len(fireworkSlots)]
		targetX := target[0] + (unitHash(j*31+event*7)-.5)*70
		targetY := target[1] + (unitHash(j*17+event*13)-.5)*50
		drawFirework(screen, startX, startY, targetX, targetY, math.Mod(local, fireworkCycle), j, event)
	}
}

const (
	burstPeony = iota
	burstRing
	burstWillow
)

func drawFirework(screen *ebiten.Image, startX, startY, targetX, targetY, local float64, seed, event int) {
	colors := fireworkColors(seed, event)
	primary := colors[0]
	white := color.RGBA{255, 250, 232, 255}
	if local < fireworkAscent {
		t := local / fireworkAscent
		at := func(t float64) (float64, float64) {
			e := 1 - (1-t)*(1-t)
			wobble := math.Sin(t*14+float64(seed)) * 6 * (1 - t)
			return startX + (targetX-startX)*e + wobble, startY + (targetY-startY)*e
		}
		// A comet tail of separate embers rather than a drawn line.
		for i := 14; i >= 1; i-- {
			px, py := at(math.Max(0, t-float64(i)*.018))
			fade := 1 - float64(i)/15
			drawGlow(screen, px, py, 2.2+3*fade, 2.2+3*fade, mix(white, primary, .5), .7*fade, ebiten.BlendLighter)
		}
		x, y := at(t)
		drawGlow(screen, x, y, 16, 16, primary, .55, ebiten.BlendLighter)
		drawGlow(screen, x, y, 4.5, 4.5, white, 1, ebiten.BlendLighter)
		return
	}
	t := local - fireworkAscent
	if t > fireworkLifetime {
		return
	}
	kind := int(unitHash(seed*53+event*29)*3) % 3
	life := t / fireworkLifetime
	fade := math.Pow(1-life, 1.3)
	// The burst briefly lights the whole sky around it.
	if t < .7 {
		sky := 1 - t/.7
		drawGlow(screen, targetX, targetY, 300, 300, primary, .2*sky*sky, ebiten.BlendLighter)
	}
	if t < .25 {
		flash := 1 - t/.25
		drawGlow(screen, targetX, targetY, 80*flash+10, 80*flash+10, white, .95*flash, ebiten.BlendLighter)
	}
	streams, drag, terminal, speedLo, speedHi, trail := 60, 1.6, 50., 300., 370., 5
	switch kind {
	case burstRing:
		streams, speedLo, speedHi, trail = 48, 340, 350, 3
	case burstWillow:
		drag, terminal, speedLo, speedHi, trail = 2, 34, 260, 350, 7
		primary = mix(primary, color.RGBA{255, 205, 110, 255}, .6)
	}
	spin := unitHash(seed*101+event*37) * math.Pi
	for s := 0; s < streams; s++ {
		// Each stream keeps one color from center to tip.
		streamColor := colors[s%len(colors)]
		if kind == burstWillow {
			streamColor = mix(streamColor, color.RGBA{255, 205, 110, 255}, .6)
		}
		angle := 2*math.Pi*float64(s)/float64(streams) + spin + (unitHash(s*13+event)-.5)*.08
		speed := speedLo + unitHash(s*43+seed*17+event)*(speedHi-speedLo)
		vx, vy := math.Cos(angle)*speed, math.Sin(angle)*speed
		for k := 0; k < trail; k++ {
			tk := t - float64(k)*.035
			if tk < 0 {
				break
			}
			decay := (1 - math.Exp(-drag*tk)) / drag
			x := targetX + vx*decay
			y := targetY + vy*decay + terminal*(tk-decay)
			tail := 1 - float64(k)/float64(trail)
			twinkle := 1.
			if life > .55 {
				twinkle = .35 + .65*math.Abs(math.Sin(t*30+float64(s)*1.7+float64(k)))
			}
			a := fade * tail * twinkle
			drawGlow(screen, x, y, 13*tail+4, 13*tail+4, streamColor, .42*a, ebiten.BlendLighter)
			drawGlow(screen, x, y, 3.6*tail+1.2, 3.6*tail+1.2, mix(white, streamColor, .3), a, ebiten.BlendLighter)
		}
	}
	if kind != burstRing && life > .5 {
		// Late crackle: glitter that blinks in and out around the burst edge.
		for g := 0; g < 36; g++ {
			blink := math.Sin(t*42 + float64(g)*2.9)
			if blink < .55 {
				continue
			}
			angle := unitHash(g*7+event*3+seed) * 2 * math.Pi
			radius := (.55 + unitHash(g*11+seed)*.5) * speedHi / drag
			x := targetX + math.Cos(angle)*radius
			y := targetY + math.Sin(angle)*radius + terminal*t*.6
			drawGlow(screen, x, y, 3, 3, white, (blink-.55)*2*fade, ebiten.BlendLighter)
		}
	}
}

var rainbow = []color.RGBA{
	{255, 49, 88, 255},  // red
	{255, 138, 50, 255}, // orange
	{255, 216, 77, 255}, // yellow
	{67, 224, 111, 255}, // green
	{49, 215, 232, 255}, // cyan
	{76, 125, 255, 255}, // blue
	{170, 92, 255, 255}, // violet
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

const (
	cannonPieces = 150
	rainPieces   = 70
	staticPieces = 44
)

// drawConfetti fires two corner cannons, then settles into a light rain. Each
// paper piece flutters by rotating and flipping, showing a darker back face.
func drawConfetti(screen *ebiten.Image, accents []color.RGBA, intro float64, motion bool) {
	palette := append([]color.RGBA{}, accents...)
	palette = append(palette, accents...)
	palette = append(palette, color.RGBA{255, 214, 92, 255}, color.RGBA{250, 250, 255, 255}, rainbow[4], rainbow[6], rainbow[3])
	var vertices []ebiten.Vertex
	var indices []uint16
	piece := func(k int, x, y, rotation, flip float64) {
		col := palette[k%len(palette)]
		shade := 1.
		if flip < 0 {
			shade = .62
		}
		w := (7 + unitHash(k*5)*5) * math.Max(.12, math.Abs(flip))
		h := 4 + unitHash(k*9)*3
		cos, sin := math.Cos(rotation), math.Sin(rotation)
		base := uint16(len(vertices))
		for _, corner := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			cx, cy := corner[0]*w/2, corner[1]*h/2
			vertices = append(vertices, ebiten.Vertex{
				DstX: float32(x + cx*cos - cy*sin), DstY: float32(y + cx*sin + cy*cos),
				SrcX: 1, SrcY: 1,
				ColorR: float32(float64(col.R) / 255 * shade), ColorG: float32(float64(col.G) / 255 * shade), ColorB: float32(float64(col.B) / 255 * shade), ColorA: 1,
			})
		}
		indices = append(indices, base, base+1, base+2, base, base+2, base+3)
	}
	if !motion {
		for k := 0; k < staticPieces; k++ {
			x := 30 + unitHash(k*3+1)*(stageW-60)
			y := 300 + unitHash(k*7+2)*230
			piece(k, x, y, unitHash(k*11)*math.Pi, .4+unitHash(k*13)*.6)
		}
	} else {
		for k := 0; k < cannonPieces; k++ {
			t := intro - confettiBlast - unitHash(k*3)*.14
			if t < 0 {
				continue
			}
			side := float64(k%2)*2 - 1 // -1 left cannon, +1 right cannon
			x0 := stageCX + side*(stageCX+20)
			angle := -math.Pi/2 - side*(.3+unitHash(k*17)*.42)
			speed := 1200 + unitHash(k*23)*700
			const drag = 2.1
			terminal := 70 + unitHash(k*29)*70
			decay := (1 - math.Exp(-drag*t)) / drag
			x := x0 + math.Cos(angle)*speed*decay + math.Sin(t*(2+unitHash(k)*2)+float64(k))*22*math.Min(1, t)
			y := screenH + 20 + math.Sin(angle)*speed*decay + terminal*(t-decay)
			if y > screenH+30 && t > 1 {
				continue
			}
			piece(k, x, y, unitHash(k*31)*6+t*(3+unitHash(k*37)*5), math.Cos(t*(5+unitHash(k*41)*7)+float64(k)))
		}
		rain := clamp01((intro - 2.2) / 1.5)
		for k := 0; k < int(float64(rainPieces)*rain); k++ {
			speed := 60 + unitHash(k*43)*60
			y := math.Mod(intro*speed+unitHash(k*47)*900, 900) - 40
			x := unitHash(k*53)*stageW + math.Sin(intro*(1+unitHash(k*59))+float64(k))*26
			piece(k+cannonPieces, x, y, intro*(2+unitHash(k*61)*3)+float64(k), math.Cos(intro*(4+unitHash(k*67)*5)+float64(k)))
		}
	}
	if len(vertices) > 0 {
		screen.DrawTriangles(vertices, indices, whitePixel, &ebiten.DrawTrianglesOptions{AntiAlias: true})
	}
}

func (m *Match) drawBanner(screen *ebiten.Image, p theme.Palette, winners []int, accents []color.RGBA, intro, loop float64, motion bool) {
	arrive := clamp01((intro - bannerArrive) / .5)
	if arrive <= 0 {
		return
	}
	accent := accents[0]
	canvas := bannerCanvas
	canvas.Clear()
	px, py := float32(panelInset), float32(panelTop)
	drawGlow(canvas, bannerW/2, panelTop+panelH/2+22, panelW*.62, panelH*.75, color.RGBA{0, 0, 0, 255}, .75, ebiten.BlendSourceOver)
	fillRoundedRect(canvas, px-4, py-4, panelW+8, panelH+8, 26, mix(accent, color.RGBA{0, 0, 0, 255}, .35))
	fillRoundedRect(canvas, px, py, panelW, panelH, 22, accent)
	panel := p.Panel
	fillRoundedRect(canvas, px+5, py+5, panelW-10, panelH-10, 18, panel)
	strokeRoundedRect(canvas, px+12, py+12, panelW-24, panelH-24, 13, 1.5, withAlpha(accent, .3))

	// Top ribbon tab.
	tabFace := &text.GoTextFace{Source: boldFont, Size: 13}
	tabText := "M A T C H   C O M P L E T E"
	tw, _ := text.Measure(tabText, tabFace, 0)
	tabW := float32(tw + 40)
	fillRoundedRect(canvas, bannerW/2-tabW/2-3, py-17, tabW+6, 34, 17, mix(accent, color.RGBA{0, 0, 0, 255}, .35))
	fillRoundedRect(canvas, bannerW/2-tabW/2, py-14, tabW, 28, 14, accent)
	drawTextCentered(canvas, tabText, tabFace, bannerW/2, float64(py), 1, 0, readableOn(accent), 1)

	title := "VICTORY!"
	if len(winners) > 1 {
		title = "TIE GAME!"
	}
	shake := drawSlamTitle(canvas, title, accents, intro, loop, motion)

	names := make([]string, len(winners))
	for i, w := range winners {
		names[i] = strings.ToUpper(m.settings.Players[w].Name)
	}
	var subtitle string
	switch {
	case len(winners) > 1:
		subtitle = strings.Join(names, " & ") + " SHARE THE WIN"
	case names[0] == "YOU":
		subtitle = "YOU WIN"
	default:
		subtitle = names[0] + " WINS"
	}
	subReveal := clamp01((intro - titleSlamStart - float64(len(title))*titleLetterStagger) / .35)
	if subReveal > 0 {
		subFace := &text.GoTextFace{Source: boldFont, Size: 24}
		sw, _ := text.Measure(subtitle, subFace, 0)
		rise := (1 - easeOutCubic(subReveal)) * 14
		crownX := bannerW/2 - sw/2 - 22
		drawCrown(canvas, crownX, panelTop+139+rise, 15, color.RGBA{255, 206, 74, 255}, subReveal)
		drawTextCentered(canvas, subtitle, subFace, bannerW/2+14, panelTop+139+rise, 1, 0, p.Text, subReveal)
		drawScoreline(canvas, p, m.settings.Players, m.state.Scores, winners, panelTop+174+rise, subReveal)
	}

	// A specular sweep crosses the card periodically, confined to its pixels.
	if motion && intro > 1.6 {
		u := math.Mod(intro-1.6, 4.5) / .9
		if u < 1 {
			sx := float64(panelInset) - 120 + u*(panelW+240)
			drawSkewBand(canvas, sx, float64(py), 70, float64(panelH), 60, .22*math.Sin(math.Pi*u))
		}
	}

	scale := .55 + .45*easeOutBack(arrive, 1.8)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-bannerW/2, -bannerH/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(stageCX+shake*.6, bannerCY-(1-easeOutCubic(arrive))*30+shake)
	op.ColorScale.ScaleAlpha(float32(clamp01(arrive * 3)))
	screen.DrawImage(canvas, op)
}

// drawSlamTitle stamps each letter in with an overshooting scale and returns
// a small vertical shake from recent impacts.
func drawSlamTitle(dst *ebiten.Image, title string, accents []color.RGBA, intro, loop float64, motion bool) float64 {
	face := &text.GoTextFace{Source: boldFont, Size: 70}
	letters := []rune(title)
	widths := make([]float64, len(letters))
	total := 0.
	const tracking = 4
	for i, r := range letters {
		widths[i], _ = text.Measure(string(r), face, 0)
		total += widths[i] + tracking
	}
	total -= tracking
	x := bannerW/2 - total/2
	shake := 0.
	const y = panelTop + 68
	dark := color.RGBA{22, 20, 34, 255}
	for i, r := range letters {
		cx := x + widths[i]/2
		x += widths[i] + tracking
		t := intro - titleSlamStart - float64(i)*titleLetterStagger
		if t < 0 {
			continue
		}
		e := clamp01(t / .3)
		scale := 2.6 - 1.6*easeOutBack(e, 2.4)
		alpha := clamp01(t / .1)
		rotation := (1 - easeOutCubic(e)) * (unitHash(i*19) - .5) * .7
		bob := 0.
		if motion {
			impact := t - .2
			if impact > 0 {
				shake += math.Exp(-impact/.06) * math.Sin(impact*85) * 3.2
			}
			bob = math.Sin(loop*3.2-float64(i)*.55) * 3 * clamp01((t-.5)/.5)
		}
		col := accents[i%len(accents)]
		glyph := string(r)
		cy := y + bob
		drawTextCentered(dst, glyph, face, cx, cy+7, scale, rotation, color.RGBA{0, 0, 0, 255}, alpha*.45)
		for k := 0; k < 16; k++ {
			a := float64(k) * math.Pi / 8
			drawTextCentered(dst, glyph, face, cx+math.Cos(a)*8, cy+math.Sin(a)*8, scale, rotation, dark, alpha)
		}
		for k := 0; k < 12; k++ {
			a := float64(k) * math.Pi / 6
			drawTextCentered(dst, glyph, face, cx+math.Cos(a)*4.5, cy+math.Sin(a)*4.5, scale, rotation, col, alpha)
		}
		drawTextCentered(dst, glyph, face, cx, cy, scale, rotation, color.RGBA{255, 251, 240, 255}, alpha)
	}
	return shake
}

func drawScoreline(dst *ebiten.Image, p theme.Palette, players []config.Player, scores []int, winners []int, y, alpha float64) {
	isWinner := map[int]bool{}
	for _, w := range winners {
		isWinner[w] = true
	}
	face := &text.GoTextFace{Source: boldFont, Size: 15}
	labels := make([]string, len(scores))
	widths := make([]float64, len(scores))
	total := 0.
	const gap, chip = 30., 18.
	for i := range scores {
		labels[i] = fmt.Sprintf("%s  %d", strings.ToUpper(players[i].Name), scores[i])
		widths[i], _ = text.Measure(labels[i], face, 0)
		total += chip + widths[i]
	}
	total += gap * float64(len(scores)-1)
	x := bannerW/2 - total/2
	for i := range scores {
		col := p.Muted
		if isWinner[i] {
			col = p.Text
		}
		vector.FillCircle(dst, float32(x+6), float32(y), 6, withAlpha(p.Players[i], alpha), true)
		drawTextCentered(dst, labels[i], face, x+chip+widths[i]/2, y, 1, 0, col, alpha)
		x += chip + widths[i] + gap
	}
}

func drawCrown(dst *ebiten.Image, cx, cy, size float64, col color.RGBA, alpha float64) {
	var path vector.Path
	s := float32(size)
	x, y := float32(cx), float32(cy)
	path.MoveTo(x-s, y+s*.6)
	path.LineTo(x-s, y-s*.5)
	path.LineTo(x-s*.5, y)
	path.LineTo(x, y-s*.75)
	path.LineTo(x+s*.5, y)
	path.LineTo(x+s, y-s*.5)
	path.LineTo(x+s, y+s*.6)
	path.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(col)
	op.ColorScale.ScaleAlpha(float32(alpha))
	vector.FillPath(dst, &path, nil, op)
}

func drawPrompt(screen *ebiten.Image, p theme.Palette, intro, loop float64, motion bool) {
	reveal := clamp01((intro - promptAppear) / .5)
	if reveal <= 0 {
		return
	}
	alpha := reveal * (.78 + .22*math.Sin(loop*3))
	face := &text.GoTextFace{Source: boldFont, Size: 13}
	keyFace := &text.GoTextFace{Source: boldFont, Size: 12}
	items := [][2]string{{"ENTER", "PLAY AGAIN"}, {"ESC", "EXIT"}}
	widths := make([]float64, len(items))
	total := 0.
	for i, item := range items {
		kw, _ := text.Measure(item[0], keyFace, 0)
		lw, _ := text.Measure(item[1], face, 0)
		widths[i] = kw + 18 + 10 + lw
		total += widths[i]
	}
	total += 34
	x := stageCX - total/2
	y := 320 + (1-easeOutCubic(reveal))*10
	light := color.RGBA{245, 246, 255, 255}
	fillRoundedRect(screen, float32(x-14), float32(y-20), float32(total+28), 40, 20, withAlpha(color.RGBA{10, 10, 20, 255}, .62*reveal))
	for i, item := range items {
		kw, _ := text.Measure(item[0], keyFace, 0)
		fillRoundedRect(screen, float32(x), float32(y-13), float32(kw+18), 26, 6, withAlpha(light, .92*alpha))
		drawTextCentered(screen, item[0], keyFace, x+(kw+18)/2, y, 1, 0, color.RGBA{24, 24, 36, 255}, alpha)
		lw, _ := text.Measure(item[1], face, 0)
		drawTextCentered(screen, item[1], face, x+kw+28+lw/2, y, 1, 0, light, alpha)
		x += widths[i] + 34
	}
	if !motion {
		drawTextCentered(screen, "REDUCED MOTION  •  PRESS M FOR FULL CELEBRATION", &text.GoTextFace{Source: boldFont, Size: 12}, stageCX, y+34, 1, 0, light, .8)
	}
}

// drawTextCentered draws text centered on (cx, cy) with scale and rotation.
func drawTextCentered(dst *ebiten.Image, value string, face *text.GoTextFace, cx, cy, scale, rotation float64, clr color.Color, alpha float64) {
	w, h := text.Measure(value, face, 0)
	op := &text.DrawOptions{}
	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(cx, cy)
	op.ColorScale.ScaleWithColor(clr)
	op.ColorScale.ScaleAlpha(float32(alpha))
	text.Draw(dst, value, face, op)
}

// drawGlow stamps the soft radial sprite as an ellipse with radii rx and ry.
func drawGlow(dst *ebiten.Image, x, y, rx, ry float64, clr color.RGBA, alpha float64, blend ebiten.Blend) {
	if alpha <= 0 || rx <= 0 || ry <= 0 {
		return
	}
	size := float64(glowImage.Bounds().Dx())
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, Blend: blend}
	op.GeoM.Translate(-size/2, -size/2)
	op.GeoM.Scale(rx*2/size, ry*2/size)
	op.GeoM.Translate(x, y)
	a := float32(math.Min(1, alpha))
	op.ColorScale.Scale(float32(clr.R)/255*a, float32(clr.G)/255*a, float32(clr.B)/255*a, a)
	dst.DrawImage(glowImage, op)
}

func drawGradientQuad(dst *ebiten.Image, x0, y0, x1, y1 float64, clr color.RGBA, topAlpha, bottomAlpha float64, blend ebiten.Blend) {
	r, g, b := float32(clr.R)/255, float32(clr.G)/255, float32(clr.B)/255
	v := func(x, y, a float64) ebiten.Vertex {
		return ebiten.Vertex{DstX: float32(x), DstY: float32(y), SrcX: 1, SrcY: 1, ColorR: r, ColorG: g, ColorB: b, ColorA: float32(a)}
	}
	vertices := []ebiten.Vertex{v(x0, y0, topAlpha), v(x1, y0, topAlpha), v(x1, y1, bottomAlpha), v(x0, y1, bottomAlpha)}
	dst.DrawTriangles(vertices, []uint16{0, 1, 2, 0, 2, 3}, whitePixel, &ebiten.DrawTrianglesOptions{Blend: blend})
}

// drawBeam draws a spotlight cone from an apex to a pool of light.
func drawBeam(dst *ebiten.Image, apexX, apexY, aimX, aimY, halfWidth float64, clr color.RGBA, alpha float64) {
	r, g, b := float32(clr.R)/255, float32(clr.G)/255, float32(clr.B)/255
	dx, dy := aimX-apexX, aimY-apexY
	length := math.Hypot(dx, dy)
	nx, ny := -dy/length*halfWidth, dx/length*halfWidth
	vertices := []ebiten.Vertex{
		{DstX: float32(apexX), DstY: float32(apexY), SrcX: 1, SrcY: 1, ColorR: r, ColorG: g, ColorB: b, ColorA: float32(alpha)},
		{DstX: float32(aimX + nx), DstY: float32(aimY + ny), SrcX: 1, SrcY: 1, ColorR: r, ColorG: g, ColorB: b, ColorA: float32(alpha * .15)},
		{DstX: float32(aimX - nx), DstY: float32(aimY - ny), SrcX: 1, SrcY: 1, ColorR: r, ColorG: g, ColorB: b, ColorA: float32(alpha * .15)},
	}
	dst.DrawTriangles(vertices, []uint16{0, 1, 2}, whitePixel, &ebiten.DrawTrianglesOptions{Blend: ebiten.BlendLighter, AntiAlias: true})
}

// drawSunburst draws slowly rotating rays that fade toward their tips.
func drawSunburst(dst *ebiten.Image, cx, cy, length, rotation float64, accents []color.RGBA, alpha float64) {
	if length <= 0 {
		return
	}
	const rays = 20
	const halfAngle = .06
	var vertices []ebiten.Vertex
	var indices []uint16
	for i := 0; i < rays; i++ {
		col := accents[i%len(accents)]
		if i%2 == 1 {
			col = mix(col, color.RGBA{255, 255, 255, 255}, .5)
		}
		r, g, b := float32(col.R)/255, float32(col.G)/255, float32(col.B)/255
		a := rotation + 2*math.Pi*float64(i)/rays
		base := uint16(len(vertices))
		vertices = append(vertices,
			ebiten.Vertex{DstX: float32(cx), DstY: float32(cy), SrcX: 1, SrcY: 1, ColorR: r, ColorG: g, ColorB: b, ColorA: float32(alpha)},
			ebiten.Vertex{DstX: float32(cx + math.Cos(a-halfAngle)*length), DstY: float32(cy + math.Sin(a-halfAngle)*length), SrcX: 1, SrcY: 1, ColorR: r, ColorG: g, ColorB: b},
			ebiten.Vertex{DstX: float32(cx + math.Cos(a+halfAngle)*length), DstY: float32(cy + math.Sin(a+halfAngle)*length), SrcX: 1, SrcY: 1, ColorR: r, ColorG: g, ColorB: b},
		)
		indices = append(indices, base, base+1, base+2)
	}
	dst.DrawTriangles(vertices, indices, whitePixel, &ebiten.DrawTrianglesOptions{Blend: ebiten.BlendLighter, AntiAlias: true})
}

// drawSkewBand lays a diagonal highlight only over existing opaque pixels.
func drawSkewBand(dst *ebiten.Image, x, y, width, height, skew, alpha float64) {
	a := float32(alpha)
	v := func(px, py float64) ebiten.Vertex {
		return ebiten.Vertex{DstX: float32(px), DstY: float32(py), SrcX: 1, SrcY: 1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: a}
	}
	vertices := []ebiten.Vertex{v(x+skew, y), v(x+skew+width, y), v(x+width, y+height), v(x, y+height)}
	dst.DrawTriangles(vertices, []uint16{0, 1, 2, 0, 2, 3}, whitePixel, &ebiten.DrawTrianglesOptions{Blend: ebiten.BlendSourceAtop, AntiAlias: true})
}

func roundedRectPath(x, y, w, h, r float32) *vector.Path {
	var path vector.Path
	path.MoveTo(x+r, y)
	path.LineTo(x+w-r, y)
	path.ArcTo(x+w, y, x+w, y+r, r)
	path.LineTo(x+w, y+h-r)
	path.ArcTo(x+w, y+h, x+w-r, y+h, r)
	path.LineTo(x+r, y+h)
	path.ArcTo(x, y+h, x, y+h-r, r)
	path.LineTo(x, y+r)
	path.ArcTo(x, y, x+r, y, r)
	path.Close()
	return &path
}

func fillRoundedRect(dst *ebiten.Image, x, y, w, h, r float32, clr color.RGBA) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, roundedRectPath(x, y, w, h, r), nil, op)
}

func strokeRoundedRect(dst *ebiten.Image, x, y, w, h, r, width float32, clr color.RGBA) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.StrokePath(dst, roundedRectPath(x, y, w, h, r), &vector.StrokeOptions{Width: width}, op)
}

// readableOn picks a dark or light ink for text placed on a colored fill.
func readableOn(bg color.RGBA) color.RGBA {
	luminance := .2126*float64(bg.R) + .7152*float64(bg.G) + .0722*float64(bg.B)
	if luminance > 150 {
		return color.RGBA{24, 22, 36, 255}
	}
	return color.RGBA{255, 255, 255, 255}
}

// withAlpha returns an opaque color at the given opacity, premultiplied as
// color.RGBA requires; assigning only the A field would brighten the color.
func withAlpha(c color.RGBA, alpha float64) color.RGBA {
	a := clamp01(alpha)
	return color.RGBA{uint8(float64(c.R)*a + .5), uint8(float64(c.G)*a + .5), uint8(float64(c.B)*a + .5), uint8(255*a + .5)}
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	lerp := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + .5) }
	return color.RGBA{lerp(a.R, b.R), lerp(a.G, b.G), lerp(a.B, b.B), lerp(a.A, b.A)}
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }
func frac(v float64) float64    { return v - math.Floor(v) }
func smoothstep(t float64) float64 {
	return t * t * (3 - 2*t)
}
func easeOutCubic(t float64) float64 { return 1 - math.Pow(1-t, 3) }
func easeOutBack(t, overshoot float64) float64 {
	return 1 + (overshoot+1)*math.Pow(t-1, 3) + overshoot*math.Pow(t-1, 2)
}
