# DotGame

DotGame is a local Dots and Boxes game written in Go. A terminal interface
configures the match, while Ebitengine provides animated gameplay. It supports
two to four seats, human and built-in AI players, variable boards, persistent
themes, and an evaluator that learns from every completed game.

## Run

```sh
go run ./cmd/dotgame
```

Select **START MATCH** and press **Enter**, or press **S** anywhere in the
terminal menu. This closes the terminal setup and opens the Ebitengine window.
To bypass the terminal menu and launch the saved configuration directly:

```sh
go run ./cmd/dotgame --play
```

The default board is 10×10 boxes with one human and one Adaptive AI. Use arrow
keys in the terminal setup, then click free edges in the game window.

Each seat can independently be Human, Easy, Medium, Hard, or Adaptive. Choose
**Watch AI Match** for a rendered zero-human game, or **Train 10 Self-Play** for
a faster headless learning batch.

### Game controls

- **Mouse:** claim an edge
- **T:** cycle through all ten themes; the choice saves immediately
- **M:** toggle full/reduced motion
- **Esc:** exit

Full motion uses a brief center-out edge reveal and a clearly visible 1.15-second
jello expansion for every captured box. Consecutive captures retain independent
animations and begin 0.25 seconds apart. At game end, the celebration waits for
the final box animation before a winner banner drops in
over a dimmed board while all ten anime-inspired layered characters form a
two-row village crowd. Their purpose-built SVG head, hair, torso, limb, and prop
layers move from explicit pivots without cropping or duplicated pixels. They
jump, cheer with signs, launch rockets, and produce looping
firework bursts. Fireworks rotate through a rainbow palette; most bursts use a
single color, while occasional two- or three-color bursts preserve one color
per detailed particle stream. Individual sparks follow curved ballistic paths
with velocity, gravity, drift, and flicker—there are no connecting lines through
the particle trails. Character arms and legs are independently articulated, and
sign carriers keep a hand aligned with the moving sign pole. Reduced motion
keeps the board transitions static and displays the darker mask, banner, signs,
and complete character crowd without continuous motion.

Settings and learned data are stored under the operating system's user config
directory in `dotgame/`. No account, network request, or paid service is used.

## Development

```sh
go test ./...
go vet ./...
```

The rules in `internal/game` are independent from rendering. AI combines
tactical move filtering, UCB-guided simulations, and a compact neural value
network whose features work across board dimensions. Choose **Train 10
Self-Play** in the terminal to run adversarial, zero-human training locally.

See `design/design-gallery.html` for the ten original visual directions.
Implementation details are in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and
[`docs/AI.md`](docs/AI.md).
