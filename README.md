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

- **Mouse:** claim an edge; hovering previews any box the edge would complete
- **T:** cycle through all ten themes; the choice saves immediately
- **M:** toggle full/reduced motion
- **Enter / R:** play again with the same seats once a match ends
- **Esc:** exit

The board sits on a lit tray whose rim glows in the active seat's color. Dots
pop in from the center when a match begins, new edges draw center-out with a
spark at each end, and every captured box snaps in with a bright 0.42-second
pop and soft settle, throwing sparks, an impact ring, and a floating +1, and is
marked with its owner's initial. Consecutive captures keep separate animations
and start 0.11 seconds apart. The
side panel shows each seat's character portrait, a territory bar, the turn
handing off between seats, and a score that pops as each box lands.

The game-end celebration starts once the last capture animation finishes. The
board turns into a night stage: the winner's territory lights up in a
shockwave, spotlights sweep the crowd, and a title card slams in one letter at
a time over a sunburst, showing the winner and the final score. All ten layered
characters spring onto the stage and dance on a shared beat. Some hop, pump
their fists, or wave; others hold placards or throw rockets from their hands
that burst into peony, ring, or willow fireworks. Corner cannons fire confetti
that flutters as it falls. Reduced motion shows the final scene without any
movement.

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
