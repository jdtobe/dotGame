# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Commands, package boundaries, asset rules, and AI constraints live in `AGENTS.md` and apply here in full:

@AGENTS.md

## Program flow

`cmd/dotgame/main.go` loads settings, stats, and the AI model (each load falls back to defaults with a warning instead of failing), then loops on `tui.Run`, which returns an `Action`:

- `Train` runs `training.Run` headlessly and returns to the TUI loop. This is the only path that goes back to the menu.
- `Play`/`Watch` save settings, build a `display.Match`, and call `display.Run` once. `Watch` copies the settings and forces every seat to AI (default difficulty `adaptive`). When the window closes, the process exits.

## Live match loop (`internal/display/display.go`)

`Match.Update` handles everything per frame:

- If the current seat is AI, it starts one goroutine that calls `agent.Choose` on `state.Clone()` and sends the result on the buffered `aiResult` channel. `aiThinking` keeps it from starting a second search.
- Human clicks go through `hitMove`. Both humans and AI apply moves through `apply`, which first appends a clone to `trajectory`, then calls `state.Apply`, then queues capture animations.
- `finish` runs once (it checks the `learned` flag). It calls `model.Learn(trajectory, scores)`, saves the model and stats, and sets `gameOverAt` after the last capture animation ends.
- After a game, Enter/R calls `restart`, which builds a new `game.State` within the same `RunGame` session. Rendering is split across three files: `display.go` (input, rules glue), `board.go` (board, sidebar, cached board layer), and `celebration.go` (game-end scene).

Headless self-play in `internal/training` follows the same trajectory → `Learn` pattern, with difficulty level `"training"`.

## Rules model (`internal/game`)

- `State` stores edges as flat `[]int8` slices: `Horizontal` is `(H+1)×W`, `Vertical` is `H×(W+1)`, and `Boxes` is `W×H`.
- Each slot holds `-1` when unclaimed, otherwise the index of the owning player.
- Width and height count boxes, not dots, and each must be between 2 and 30. A game has 2–4 players.
- `Apply` returns a `Turn` that lists the completed box indices and whether the player gets an extra turn.

## Persistence

All files live under `config.Dir()`, which is `os.UserConfigDir()/dotgame`:

- `settings.json` and `stats.json` use `config.currentVersion`.
- `ai-model.json` uses `ai.modelVersion`. If the version or the layer dimensions don't match, `LoadModel` silently returns a fresh model.

`Model` is shared across goroutines and protected by its internal `sync.RWMutex`.
