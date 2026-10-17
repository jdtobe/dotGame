// Package tui implements the terminal setup and statistics interface.
package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jdtobe/dotGame/internal/ai"
	"github.com/jdtobe/dotGame/internal/config"
	"github.com/jdtobe/dotGame/internal/theme"
)

// Action identifies what the terminal menu asked the application to do.
type Action uint8

const (
	Quit Action = iota
	Play
	Watch
	Train
)

type model struct {
	settings                  config.Settings
	stats                     config.Stats
	learnedGames, positions   int
	decisions, searches       int
	cursor, width, height     int
	start, watch, train, quit bool
	message                   string
}

// Run displays setup and returns the selected match configuration.
func Run(settings config.Settings, stats config.Stats, learned *ai.Model) (config.Settings, Action, error) {
	g, p := learned.Snapshot()
	d, s := learned.SearchMetrics()
	m := model{settings: settings, stats: stats, learnedGames: g, positions: p, decisions: d, searches: s}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return settings, Quit, err
	}
	r := final.(model)
	if r.train {
		return r.settings, Train, nil
	}
	if r.watch {
		return r.settings, Watch, nil
	}
	if r.start {
		return r.settings, Play, nil
	}
	return r.settings, Quit, nil
}

func (m model) Init() tea.Cmd { return nil }
func (m model) rows() int     { return 9 + len(m.settings.Players) }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch x := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = x.Width
		m.height = x.Height
	case tea.KeyMsg:
		switch x.String() {
		case "q", "ctrl+c":
			m.quit = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < m.rows()-1 {
				m.cursor++
			}
		case "left", "h":
			m = m.change(-1)
		case "right", "l":
			m = m.change(1)
		case " ":
			if m.cursor >= m.rows()-3 {
				return m.activate()
			}
			m = m.change(1)
		case "enter":
			return m.activate()
		case "s":
			m.cursor = m.rows() - 3
			return m.activate()
		case "w":
			m.cursor = m.rows() - 2
			return m.activate()
		}
	}
	return m, nil
}

func (m model) activate() (tea.Model, tea.Cmd) {
	if m.cursor == m.rows()-3 {
		if hasHuman(m.settings.Players) {
			m.start = true
			_ = config.SaveSettings(m.settings)
			return m, tea.Quit
		}
		m.message = "A standard game needs at least one human seat."
	}
	if m.cursor == m.rows()-2 {
		m.watch = true
		return m, tea.Quit
	}
	if m.cursor == m.rows()-1 {
		m.train = true
		_ = config.SaveSettings(m.settings)
		return m, tea.Quit
	}
	return m, nil
}

func hasHuman(ps []config.Player) bool {
	for _, p := range ps {
		if p.Kind == config.Human {
			return true
		}
	}
	return false
}
func (m model) change(delta int) model {
	switch {
	case m.cursor == 0:
		m.settings.Width = clamp(m.settings.Width+delta, 2, 20)
	case m.cursor == 1:
		m.settings.Height = clamp(m.settings.Height+delta, 2, 20)
	case m.cursor == 2:
		n := clamp(len(m.settings.Players)+delta, 2, 4)
		for len(m.settings.Players) < n {
			i := len(m.settings.Players)
			m.settings.Players = append(m.settings.Players, config.Player{Name: fmt.Sprintf("Player %d", i+1), Kind: config.AI, Difficulty: "medium"})
		}
		m.settings.Players = m.settings.Players[:n]
	case m.cursor >= 3 && m.cursor < 3+len(m.settings.Players):
		i := m.cursor - 3
		cyclePlayer(&m.settings.Players[i], delta)
	case m.cursor == 3+len(m.settings.Players):
		levels := []string{"easy", "medium", "hard", "adaptive"}
		for i, p := range levels {
			if p == aiLevel(m.settings.Players) {
				n := (i + delta + len(levels)) % len(levels)
				for j := range m.settings.Players {
					if m.settings.Players[j].Kind == config.AI {
						m.settings.Players[j].Difficulty = levels[n]
					}
				}
				break
			}
		}
	case m.cursor == 4+len(m.settings.Players):
		idx := 0
		for i, p := range theme.All {
			if p.ID == m.settings.Theme {
				idx = i
			}
		}
		m.settings.Theme = theme.All[(idx+delta+len(theme.All))%len(theme.All)].ID
		_ = config.SaveSettings(m.settings)
	case m.cursor == 5+len(m.settings.Players):
		m.settings.Animations = !m.settings.Animations
		_ = config.SaveSettings(m.settings)
	}
	return m
}

func cyclePlayer(player *config.Player, delta int) {
	levels := []string{"human", "easy", "medium", "hard", "adaptive"}
	current := "human"
	if player.Kind == config.AI {
		current = player.Difficulty
	}
	index := 0
	for i, level := range levels {
		if level == current {
			index = i
			break
		}
	}
	next := levels[(index+delta+len(levels))%len(levels)]
	if next == "human" {
		player.Kind = config.Human
		player.Difficulty = ""
		return
	}
	player.Kind = config.AI
	player.Difficulty = next
}
func aiLevel(ps []config.Player) string {
	for _, p := range ps {
		if p.Kind == config.AI && p.Difficulty != "" {
			return p.Difficulty
		}
	}
	return "adaptive"
}
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func ansi(c color.RGBA) lipgloss.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B))
}
func (m model) View() string {
	if m.quit {
		return ""
	}
	p := theme.ByID(m.settings.Theme)
	title := lipgloss.NewStyle().Bold(true).Foreground(ansi(p.Accent)).Render("DOTGAME  /  BUILD YOUR TABLE")
	dim := lipgloss.NewStyle().Foreground(ansi(p.Muted))
	label := lipgloss.NewStyle().Foreground(ansi(p.Text))
	selected := lipgloss.NewStyle().Bold(true).Foreground(ansi(p.Background)).Background(ansi(p.Accent)).Padding(0, 1)
	var rows []string
	add := func(name, value string) {
		i := len(rows)
		line := fmt.Sprintf(" %-19s ◀ %-17s ▶", name, value)
		if i == m.cursor {
			line = selected.Render(line)
		} else {
			line = label.Render(line)
		}
		rows = append(rows, line)
	}
	addAction := func(name, shortcut string) {
		i := len(rows)
		line := fmt.Sprintf(" %-29s %-13s", name, shortcut)
		if i == m.cursor {
			line = selected.Render(line)
		} else {
			line = label.Render(line)
		}
		rows = append(rows, line)
	}
	add("Board width", fmt.Sprintf("%d boxes", m.settings.Width))
	add("Board height", fmt.Sprintf("%d boxes", m.settings.Height))
	add("Seats", fmt.Sprintf("%d players", len(m.settings.Players)))
	for i, player := range m.settings.Players {
		kind := strings.ToUpper(string(player.Kind))
		if player.Kind == config.AI {
			kind += " · " + strings.ToUpper(player.Difficulty)
		}
		add(fmt.Sprintf("%d  %s", i+1, player.Name), kind)
	}
	add("AI strength", strings.ToUpper(aiLevel(m.settings.Players)))
	add("Theme", theme.ByID(m.settings.Theme).Name)
	if m.settings.Animations {
		add("Animation", "FULL")
	} else {
		add("Animation", "REDUCED")
	}
	addAction("START MATCH", "[ S / ENTER ]")
	addAction("WATCH AI MATCH", "[ W / ENTER ]")
	addAction("TRAIN 10 SELF-PLAY", "[ ENTER ]")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ansi(p.Line)).Padding(1, 2).Width(54).Render(strings.Join(rows, "\n"))
	stats := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ansi(p.Line)).Padding(1, 2).Width(33).Render(fmt.Sprintf("%s\n\nGames played      %d\nTies              %d\nMoves observed    %d\nBoxes captured    %d\nAverage moves     %.1f\n\n%s\nTraining games    %d\nLearned positions %d\nAI decisions      %d\nSearch iterations %d\nNeural value      ONLINE", lipgloss.NewStyle().Bold(true).Foreground(ansi(p.Accent)).Render("LIFETIME STATS"), m.stats.Games, m.stats.Ties, m.stats.TotalMoves, m.stats.Boxes, average(m.stats.TotalMoves, m.stats.Games), lipgloss.NewStyle().Bold(true).Foreground(ansi(p.Accent)).Render("AI CAPABILITY"), m.learnedGames, m.positions, m.decisions, m.searches))
	body := lipgloss.JoinHorizontal(lipgloss.Top, box, "  ", stats)
	help := dim.Render("↑/↓ navigate   ←/→ change   S start game   W watch AI   q quit")
	msg := ""
	if m.message != "" {
		msg = "\n" + lipgloss.NewStyle().Foreground(ansi(p.Players[0])).Render(m.message)
	}
	return lipgloss.NewStyle().Padding(1, 3).Render(title + "\n" + dim.Render("Terminal setup · theme changes save immediately") + "\n\n" + body + "\n\n" + help + msg)
}

func average(total, count int) float64 {
	if count == 0 {
		return 0
	}
	return float64(total) / float64(count)
}
