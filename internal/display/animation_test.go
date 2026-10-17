package display

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestCapturedBoxesReceiveStaggeredPersistentAnimations(t *testing.T) {
	match := &Match{captureAnimations: map[int]captureAnimation{}}
	start := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	match.scheduleCaptures([]int{7, 8}, start)
	// A fast extra turn must queue behind the existing pair instead of
	// replacing them or restarting the schedule.
	match.scheduleCaptures([]int{9}, start.Add(100*time.Millisecond))
	want := map[int]time.Time{
		7: start,
		8: start.Add(250 * time.Millisecond),
		9: start.Add(500 * time.Millisecond),
	}
	if len(match.captureAnimations) != len(want) {
		t.Fatalf("retained %d animations, want %d", len(match.captureAnimations), len(want))
	}
	for box, expected := range want {
		if got := match.captureAnimations[box].start; !got.Equal(expected) {
			t.Errorf("box %d starts at %s, want %s", box, got, expected)
		}
	}
}

func TestJelloScaleExpandsAndSettles(t *testing.T) {
	x, y := jelloScale(0)
	if x > .05 || y > .05 {
		t.Fatalf("capture should begin at the center: %.3f %.3f", x, y)
	}
	overshot := false
	deformed := false
	for i := 1; i < int(captureDuration*100); i++ {
		x, y = jelloScale(float64(i) / 100)
		if x > 1 || y > 1 {
			overshot = true
		}
		if x < 0 || y < 0 {
			t.Fatalf("capture scale became negative: %.3f %.3f", x, y)
		}
		if x-y > .1 || y-x > .1 {
			deformed = true
		}
	}
	if !overshot {
		t.Fatal("capture animation never overshot its final size")
	}
	if !deformed {
		t.Fatal("capture animation never visibly squashed or stretched")
	}
	x, y = jelloScale(captureDuration)
	if x != 1 || y != 1 {
		t.Fatalf("capture should settle exactly at full size: %.3f %.3f", x, y)
	}
}

func TestAllCharacterAssetsLoad(t *testing.T) {
	characters, err := loadCharacterSprites()
	if err != nil {
		t.Fatal(err)
	}
	if len(characters) != 10 {
		t.Fatalf("loaded %d characters, want 10", len(characters))
	}
	for _, character := range characters {
		parts := []*ebiten.Image{character.backHair, character.leftLeg, character.rightLeg, character.leftArm, character.rightArm, character.body, character.head, character.prop}
		for _, part := range parts {
			if part == nil || part.Bounds().Dx() != 128 || part.Bounds().Dy() != 140 {
				t.Fatalf("%s has invalid articulated image dimensions", character.name)
			}
		}
	}
}

func TestFireworkPaletteUsesMostlySingleColorBursts(t *testing.T) {
	single, total := 0, 0
	for seed := 0; seed < 10; seed++ {
		for event := 0; event < 12; event++ {
			colors := fireworkColors(seed, event)
			if len(colors) == 1 {
				single++
			}
			if len(colors) < 1 || len(colors) > 3 {
				t.Fatalf("firework has %d colors, want 1–3", len(colors))
			}
			total++
		}
	}
	if single*4 < total*3 {
		t.Fatalf("only %d/%d fireworks are single-color", single, total)
	}
}

func TestConsecutiveLaunchersCoverRainbow(t *testing.T) {
	seen := map[[3]uint8]bool{}
	for seed := 0; seed < len(rainbow); seed++ {
		color := fireworkColors(seed, 0)[0]
		seen[[3]uint8{color.R, color.G, color.B}] = true
	}
	if len(seen) != len(rainbow) {
		t.Fatalf("first seven launchers produced %d distinct rainbow colors", len(seen))
	}
}
