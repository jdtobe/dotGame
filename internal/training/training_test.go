package training

import (
	"context"
	"testing"

	"github.com/jdtobe/dotGame/internal/ai"
	"github.com/jdtobe/dotGame/internal/config"
)

func TestRunCompletesAndLearns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	settings := config.Defaults()
	settings.Width, settings.Height = 2, 2
	model := ai.NewModel()
	progress := 0
	if err := Run(context.Background(), settings, model, 1, func(done int) { progress = done }); err != nil {
		t.Fatal(err)
	}
	games, positions := model.Snapshot()
	if progress != 1 || games != 1 || positions == 0 {
		t.Fatalf("progress=%d games=%d positions=%d", progress, games, positions)
	}
}
