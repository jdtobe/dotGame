// Package training runs local, zero-human self-play sessions.
package training

import (
	"context"
	"fmt"

	"github.com/jdtobe/dotGame/internal/ai"
	"github.com/jdtobe/dotGame/internal/config"
	"github.com/jdtobe/dotGame/internal/game"
)

// Run performs count headless self-play games and reports completed games.
// Training boards are capped at 6×6 so a CPU-only batch completes promptly.
func Run(ctx context.Context, settings config.Settings, model *ai.Model, count int, progress func(done int)) error {
	w, h := min(settings.Width, 6), min(settings.Height, 6)
	agent := ai.New(model)
	for n := 0; n < count; n++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		s, err := game.New(w, h, len(settings.Players))
		if err != nil {
			return err
		}
		var trajectory []*game.State
		for !s.Done() {
			trajectory = append(trajectory, s.Clone())
			mv, ok := agent.Choose(ctx, s.Clone(), "training")
			if !ok {
				return fmt.Errorf("self-play agent found no move with %d edges remaining", s.Remaining())
			}
			if _, err = s.Apply(mv); err != nil {
				return err
			}
		}
		model.Learn(trajectory, s.Scores)
		if progress != nil {
			progress(n + 1)
		}
	}
	return model.Save()
}
