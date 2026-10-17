package display

import (
	"math"
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
		8: start.Add(captureStagger),
		9: start.Add(2 * captureStagger),
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

func TestCaptureScalePopsAndSettlesQuickly(t *testing.T) {
	if s := captureScale(0); s > .55 {
		t.Fatalf("capture should begin visibly smaller than the box: %.3f", s)
	}
	if captureDuration > .5 {
		t.Fatalf("capture animation lasts %.2fs; keep it snappy", captureDuration)
	}
	peak := 0.
	previous := captureScale(0)
	for i := 1; i <= 100; i++ {
		age := captureDuration * float64(i) / 100
		s := captureScale(age)
		peak = math.Max(peak, s)
		if age <= capturePop && s < previous {
			t.Fatalf("pop should grow monotonically, shrank at %.3fs", age)
		}
		if s < .95 && age > capturePop {
			t.Fatalf("settle undershot to %.3f at %.3fs", s, age)
		}
		if math.Abs(s-previous) > .25 {
			t.Fatalf("scale jumped %.3f at %.3fs", s-previous, age)
		}
		previous = s
	}
	if peak <= 1.05 {
		t.Fatalf("capture never overshot its final size: peak %.3f", peak)
	}
	if s := captureScale(captureDuration); s != 1 {
		t.Fatalf("capture should settle exactly at full size: %.3f", s)
	}
	if boxLiveFor < captureDuration || boxLiveFor < captureSparkle {
		t.Fatal("a box must not be cached before its capture effects finish")
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
			if part == nil || part.Bounds().Dx() != layerWidth*layerSupersample || part.Bounds().Dy() != layerHeight*layerSupersample {
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
