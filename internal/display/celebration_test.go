package display

import (
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/jdtobe/dotGame/internal/config"
	"github.com/jdtobe/dotGame/internal/game"
)

func TestRestartBeginsFreshMatch(t *testing.T) {
	s, err := game.New(3, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	for !s.Done() {
		if _, err := s.Apply(s.LegalMoves()[0]); err != nil {
			t.Fatal(err)
		}
	}
	match := &Match{state: s, learned: true, gameOverAt: time.Now(), trajectory: []*game.State{s.Clone()}, captureAnimations: map[int]captureAnimation{1: {start: time.Now()}}, nextCaptureAt: time.Now(), hasLastMove: true}
	match.restart()
	if match.state.Done() || match.state.Remaining() == 0 || match.state.Width != 3 || match.state.Height != 2 || match.state.Players != 2 {
		t.Fatalf("restart did not create an equivalent empty board: %+v", match.state)
	}
	if match.learned || !match.gameOverAt.IsZero() || len(match.trajectory) != 0 || len(match.captureAnimations) != 0 || !match.nextCaptureAt.IsZero() || match.hasLastMove {
		t.Fatal("restart kept state from the completed match")
	}
	if match.startedAt.IsZero() {
		t.Fatal("restart should replay the board intro")
	}
}

func TestWithAlphaPremultiplies(t *testing.T) {
	got := withAlpha(color.RGBA{200, 100, 50, 255}, .5)
	want := color.RGBA{100, 50, 25, 128}
	if got != want {
		t.Fatalf("withAlpha = %v, want %v", got, want)
	}
	if c := withAlpha(color.RGBA{255, 255, 255, 255}, 2); c.A != 255 || c.R != 255 {
		t.Fatalf("alpha above one should clamp: %v", c)
	}
}

func TestCrowdEntersFromCenterAndSettlesWithoutMotion(t *testing.T) {
	match := &Match{characters: make([]characterSprite, 10)}
	for i, pose := range match.crowdPoses(crowdEnterStart-.01, 0, true) {
		if pose.visible {
			t.Fatalf("character %d appeared before the crowd entrance", i)
		}
	}
	center := crowdDelay(7) // front-row center
	for i := range match.characters {
		if crowdDelay(i) < center {
			t.Fatalf("character %d enters before the front-row center", i)
		}
	}
	for i, pose := range match.crowdPoses(60, 0, false) {
		if !pose.visible || pose.jump != 0 || pose.squash != 0 || pose.tilt != 0 {
			t.Fatalf("reduced-motion character %d is not at rest: %+v", i, pose)
		}
	}
}

func TestDanceKeepsRigInRange(t *testing.T) {
	for role := roleHop; role <= roleRocket; role++ {
		for step := 0; step < 800; step++ {
			beat := float64(step) / 50
			pose := rigPose{role: role, launcher: 0}
			dance(&pose, beat, step%10, beat*celebrationBeat+launchStart(0))
			if pose.armL < -.5 || pose.armL > 1.5 || pose.armR < -.9 || pose.armR > .6 {
				t.Fatalf("role %d beat %.2f moved arms out of the rig range: %+v", role, beat, pose)
			}
			if math.Abs(pose.legL) > .3 || math.Abs(pose.legR) > .3 || math.Abs(pose.squash) > .2 || math.Abs(pose.tilt) > .1 {
				t.Fatalf("role %d beat %.2f distorted the body: %+v", role, beat, pose)
			}
			if pose.jump < 0 || pose.jump > 40 {
				t.Fatalf("role %d beat %.2f jumped %.1f", role, beat, pose.jump)
			}
		}
	}
}

func TestSignGripFollowsLeftHand(t *testing.T) {
	pose := rigPose{x: 300, ground: 600, scale: 1, armL: 1.05}
	x, y := pose.leftHand()
	pose.armL = 0
	restX, restY := pose.leftHand()
	if y >= restY || math.Abs(x-restX) < 1 {
		t.Fatalf("raising the arm should lift the hand: rest (%.1f,%.1f) raised (%.1f,%.1f)", restX, restY, x, y)
	}
}

func TestFireworkBurstsClearTheTitleCard(t *testing.T) {
	// The card occupies this screen rectangle at full scale.
	left, right := float64(stageCX-panelW/2), float64(stageCX+panelW/2)
	top, bottom := float64(bannerCY-bannerH/2+panelTop), float64(bannerCY-bannerH/2+panelTop+panelH)
	for _, slot := range fireworkSlots {
		for _, dx := range []float64{-35, 35} {
			for _, dy := range []float64{-25, 25} {
				x, y := slot[0]+dx, slot[1]+dy
				if x > left && x < right && y > top && y < bottom {
					t.Fatalf("firework slot %v can burst behind the title card", slot)
				}
			}
		}
	}
}

func TestSignMessagesFitWinner(t *testing.T) {
	match := &Match{settings: config.Settings{Players: []config.Player{{Name: "Extraordinary"}, {Name: "Ada"}}}}
	for _, message := range match.signMessages([]int{0}) {
		if len([]rune(message)) > 8 {
			t.Fatalf("sign message %q is too long for the placard", message)
		}
	}
	if got := match.signMessages([]int{0, 1}); got[1] != "TIE!" {
		t.Fatalf("tie signs = %v", got)
	}
}
