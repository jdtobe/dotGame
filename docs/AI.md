# AI and continual learning

DotGame's AI is designed to run locally on an ordinary CPU. It does not call a
hosted model or require an account.

## Decision pipeline

1. **Tactical filtering** immediately takes available boxes and avoids creating
   a third side around a box while safe moves remain.
2. **UCB-guided Monte Carlo search** compares the remaining candidates. Search
   follows real extra-turn and multiplayer rules rather than reducing every
   match to a two-player board.
3. A compact **6→12→1 neural value network** estimates unfinished positions
   when a rollout reaches its depth budget. Features are normalized, so one
   model can evaluate different board dimensions.

Difficulty levels change the search budget. The tactical layer remains active
at every level so an AI does not deliberately ignore an immediately available
box merely to appear easier.

## Learning

Each position visited in a completed match is paired with its eventual result.
The value network is updated through online backpropagation after the match, so
the model never changes halfway through a decision. Human games, watched AI
games, and headless self-play all contribute examples.

The terminal's **Train 10 Self-Play** action runs zero-human adversarial games.
Training boards are capped at 6×6 to keep a batch practical on a home computer;
normalized features transfer what is learned to larger boards. Model weights,
games, positions, decisions, and search iterations are saved atomically in the
user configuration directory.

## Practical limitations

This is intentionally smaller than AlphaZero. It has no GPU dependency, large
replay cluster, or deep residual policy network. Strength comes from combining
domain tactics and search with a continually improving evaluator. The persisted
model format is versioned; incompatible experimental models are safely replaced
with a fresh model rather than loaded incorrectly.
