package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/jdtobe/dotGame/internal/ai"
	"github.com/jdtobe/dotGame/internal/config"
	"github.com/jdtobe/dotGame/internal/display"
	"github.com/jdtobe/dotGame/internal/training"
	"github.com/jdtobe/dotGame/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "dotgame:", err)
		os.Exit(1)
	}
}
func run() error {
	settings, err := config.LoadSettings()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	stats, err := config.LoadStats()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	model, err := ai.LoadModel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: AI model reset:", err)
	}
	if hasArg("--play") {
		match, matchErr := display.New(settings, stats, model)
		if matchErr != nil {
			return matchErr
		}
		if err = display.Run(match); errors.Is(err, ebiten.Termination) {
			return nil
		}
		return err
	}
	for {
		var action tui.Action
		settings, action, err = tui.Run(settings, stats, model)
		if err != nil {
			return err
		}
		switch action {
		case tui.Quit:
			return nil
		case tui.Train:
			fmt.Println("Running 10 local self-play games...")
			err = training.Run(context.Background(), settings, model, 10, func(done int) { fmt.Printf("\rTraining: %d/10", done) })
			fmt.Println()
			if err != nil {
				return err
			}
			continue
		case tui.Play, tui.Watch:
			if err = config.SaveSettings(settings); err != nil {
				return err
			}
			matchSettings := settings
			matchSettings.Players = append([]config.Player(nil), settings.Players...)
			if action == tui.Watch {
				for i := range matchSettings.Players {
					matchSettings.Players[i].Kind = config.AI
					if matchSettings.Players[i].Difficulty == "" {
						matchSettings.Players[i].Difficulty = "adaptive"
					}
				}
			}
			match, matchErr := display.New(matchSettings, stats, model)
			if matchErr != nil {
				return matchErr
			}
			err = display.Run(match)
			if errors.Is(err, ebiten.Termination) {
				return nil
			}
			return err
		}
	}
}

func hasArg(want string) bool {
	for _, arg := range os.Args[1:] {
		if arg == want {
			return true
		}
	}
	return false
}
