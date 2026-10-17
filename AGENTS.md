# Repository guidance

## Commands

- Requires Go 1.25 (`go.mod`). Run with `go run ./cmd/dotgame`; use `--play` to bypass the TUI and open the saved match directly.
- Before finishing: `gofmt -w cmd internal`, `go test ./...`, and `go vet ./...`. Run races with `go test -race ./...` (or a focused package such as `go test -race ./internal/display`).
- Run one test with `go test ./internal/display -run TestName`. `make check` runs tests and vet but not formatting or the race detector.
- `make build` writes `bin/dotgame`; a root-level `dotgame` binary is intentionally ignored.

## Execution and package boundaries

- `cmd/dotgame` runs Bubble Tea first, then calls Ebitengine once. Keep `ebiten.RunGame` on the main path; do not add a second RunGame cycle to return to the TUI. `--play` is the useful GUI-only smoke path.
- `internal/game` is the deterministic rules source and must stay UI-independent. Only the Ebitengine `Update` loop mutates the live match; AI/search operates on cloned states in a goroutine.
- `internal/training` caps self-play boards at 6×6 for CPU practicality even when the configured board is larger.
- Settings, statistics, and the AI model live in `os.UserConfigDir()/dotgame`, not the repository. Tests that save data must use `t.Setenv("HOME", t.TempDir())` to avoid changing the developer's real profile.
- Persistence formats are versioned separately in `internal/config/config.go` and `internal/ai/model.go`; update validation/migration behavior when changing serialized fields.

## Rendering assets and animation

- Runtime character art is the 80 SVG files under `internal/display/assets/stylized`, embedded and rasterized by `characters.go`. Each character must provide exactly: `back-hair`, `leg-left`, `leg-right`, `arm-left`, `arm-right`, `body`, `head-group`, and `prop` layers.
- Never recreate character layers by cropping a flattened image: that previously caused duplicated outlines and cut-off parts. Props share the right-arm pivot; back hair shares the head pivot. Runtime layers rasterize to 128×140, and pivots in `drawVillage` assume the 320×350 source viewBox at 0.4 scale.
- The HTML files under `design/` are standalone concept galleries, not runtime inputs. Editing a gallery does not regenerate embedded SVG assets; update both deliberately.
- Captured boxes retain independent animation records. Preserve the 250 ms queue stagger and delay the victory overlay until the final 1.15 s capture animation completes; do not replace the queue with a single “latest capture” value.
- `internal/theme/theme.go` is the shared palette registry. Hover colors intentionally differ from every player color, and intersection dots are intentionally dim.

## AI constraints

- AI strength combines tactical filtering, UCB-guided simulations, and the persisted 6→12→1 value network; difficulty changes search budget, not core legality/tactical capture behavior.
- Learning occurs after completed games, never during an in-progress decision. Human, watched-AI, and headless self-play games all feed the same model.
