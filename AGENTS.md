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
- Never recreate character layers by cropping a flattened image: that previously caused duplicated outlines and cut-off parts. Props share the right-arm pivot; back hair shares the head pivot. Rig coordinates are 128×140 layer units (the 320×350 source viewBox at 0.4). Layers rasterize at `layerSupersample`× that size and draw with linear filtering through `rigPose.rigGeoM`, which `leftHand`/`rightHand` reuse so placards and rockets stay attached to hands.
- The HTML files under `design/` are standalone concept galleries, not runtime inputs. Editing a gallery does not regenerate embedded SVG assets; update both deliberately.
- Captured boxes retain independent animation records. Preserve the queue stagger (`captureStagger`, 110 ms) and delay the victory overlay until the final capture animation (`captureDuration`, 0.42 s) completes; do not replace the queue with a single “latest capture” value.
- The celebration (`celebration.go`) is a pure function of time since `gameOverAt`. Reduced motion evaluates the settled end state with the looping clock at zero, so don't add per-frame state to it.
- `board.go` caches the settled board in an offscreen layer that is redrawn only when `boardLayerKey` changes. Anything new that affects the static board must go into that key, or the layer goes stale.
- `color.RGBA` is premultiplied. Use `withAlpha` to fade a color; setting only `.A` brightens it instead of making it translucent.
- `internal/theme/theme.go` is the shared palette registry. Hover colors intentionally differ from every player color, and intersection dots are intentionally dim.

## AI constraints

- AI strength combines tactical filtering, UCB-guided simulations, and the persisted 6→12→1 value network; difficulty changes search budget, not core legality/tactical capture behavior.
- Learning occurs after completed games, never during an in-progress decision. Human, watched-AI, and headless self-play games all feed the same model.
