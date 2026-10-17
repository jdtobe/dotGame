package theme

import (
	"math"
	"testing"
)

func TestHoverColorsAreDistinctFromPlayers(t *testing.T) {
	for _, palette := range All {
		for i, player := range palette.Players {
			dr := float64(int(palette.Hover.R) - int(player.R))
			dg := float64(int(palette.Hover.G) - int(player.G))
			db := float64(int(palette.Hover.B) - int(player.B))
			if distance := math.Sqrt(dr*dr + dg*dg + db*db); distance < 70 {
				t.Errorf("%s hover is too close to player %d (distance %.1f)", palette.Name, i+1, distance)
			}
		}
	}
}
