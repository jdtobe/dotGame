# Architecture

- `cmd/dotgame` composes persistence, the terminal menu, training, and display.
- `internal/game` is the deterministic rules engine and has no UI dependency.
- `internal/ai` owns tactics, search, neural evaluation, and model persistence.
- `internal/training` runs cancellable zero-human self-play batches.
- `internal/tui` presents setup, statistics, themes, and AI controls.
- `internal/display` owns Ebitengine rendering, animation, and pointer input.
- `internal/config` atomically persists settings and lifetime statistics.
- `internal/theme` is the shared source of all ten palettes.

The renderer and AI receive clones of game state where appropriate. Only the
Ebitengine update loop mutates the live match, which keeps rendering responsive
and avoids concurrent board access. Rules tests can run without initializing a
window or terminal.
