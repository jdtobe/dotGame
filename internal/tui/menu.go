// Package tui implements the terminal setup and statistics interface.
package tui

import (
	"fmt"
	"image/color"
	"sort"
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
	fg := func(c color.RGBA) lipgloss.Style { return lipgloss.NewStyle().Foreground(ansi(c)) }
	dim, text, accent := fg(p.Muted), fg(p.Text), fg(p.Accent).Bold(true)
	selected := lipgloss.NewStyle().Bold(true).Foreground(ansi(p.Background)).Background(ansi(p.Accent))
	const rowWidth = 50

	// Wordmark: a tiny claimed box drawn with the first two seat colors.
	edgeA, edgeB := fg(p.Players[0]).Bold(true), fg(p.Players[1]).Bold(true)
	dot := text.Render("●")
	logo := []string{
		dot + edgeA.Render("━━━") + dot + "    " + accent.Render("D O T G A M E"),
		edgeB.Render("┃") + fg(p.Players[0]).Render("▒▒▒") + edgeA.Render("┃") + "    " + dim.Render("Build your table"),
		dot + edgeB.Render("━━━") + dot,
	}

	var lines []string
	row := 0
	section := func(name string) {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, dim.Render(strings.ToUpper(name)))
	}
	// setting renders one adjustable row; marker is an optional colored glyph.
	setting := func(name, value, marker string, markerColor color.RGBA) {
		if row == m.cursor {
			body := fmt.Sprintf(" ▸ %-17s‹ %s ›", name, value)
			if marker != "" {
				body = fmt.Sprintf(" ▸ %s %-15s‹ %s ›", marker, name, value)
			}
			lines = append(lines, selected.Render(fmt.Sprintf("%-*s", rowWidth, body)))
		} else {
			lead := "   "
			if marker != "" {
				lead = "   " + fg(markerColor).Render(marker) + " "
				name = fmt.Sprintf("%-15s", name)
			} else {
				name = fmt.Sprintf("%-17s", name)
			}
			lines = append(lines, lead+dim.Render(name)+"  "+text.Render(value))
		}
		row++
	}
	action := func(name, key string, primary bool) {
		label := fmt.Sprintf("  %-*s%s  ", rowWidth-len(key)-4, name, key)
		switch {
		case row == m.cursor:
			lines = append(lines, selected.Render(label))
		case primary:
			lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ansi(p.Accent)).Render(label))
		default:
			lines = append(lines, text.Render(label))
		}
		row++
	}

	section("Table")
	setting("Board width", fmt.Sprintf("%d boxes", m.settings.Width), "", p.Text)
	setting("Board height", fmt.Sprintf("%d boxes", m.settings.Height), "", p.Text)
	setting("Seats", fmt.Sprintf("%d players", len(m.settings.Players)), "", p.Text)
	section("Players")
	for i, player := range m.settings.Players {
		kind := "HUMAN"
		if player.Kind == config.AI {
			kind = "AI · " + strings.ToUpper(player.Difficulty)
		}
		setting(player.Name, kind, "●", p.Players[i%len(p.Players)])
	}
	setting("AI strength", strings.ToUpper(aiLevel(m.settings.Players)), "", p.Text)
	section("Presentation")
	swatch := ""
	for _, c := range p.Players {
		swatch += fg(c).Render("■")
	}
	setting("Theme", p.Name, "", p.Text)
	// The swatch follows unselected rows only, so selection styling stays intact.
	if row-1 != m.cursor {
		lines[len(lines)-1] += "  " + swatch
	}
	motion := "FULL"
	if !m.settings.Animations {
		motion = "REDUCED"
	}
	setting("Motion", motion, "", p.Text)
	lines = append(lines, "")
	action("▶  START MATCH", "S", true)
	action("◉  WATCH AI MATCH", "W", false)
	action("↻  TRAIN 10 SELF-PLAY", "ENTER", false)

	panel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ansi(p.Line)).Padding(1, 2)
	setup := panel.Width(rowWidth + 6).Render(strings.Join(lines, "\n"))

	stat := func(name string, value any) string {
		return dim.Render(fmt.Sprintf("%-18s", name)) + text.Bold(true).Render(fmt.Sprint(value))
	}
	statLines := []string{
		accent.Render("LIFETIME"),
		stat("Games played", m.stats.Games),
		stat("Ties", m.stats.Ties),
		stat("Boxes captured", m.stats.Boxes),
		stat("Average moves", fmt.Sprintf("%.1f", average(m.stats.TotalMoves, m.stats.Games))),
	}
	if leaders := topWinners(m.stats.Wins, 3); len(leaders) > 0 {
		statLines = append(statLines, "", accent.Render("TOP WINNERS"))
		most := leaders[0].wins
		for i, l := range leaders {
			bar := strings.Repeat("█", max(1, l.wins*10/most))
			name := l.name
			if len([]rune(name)) > 9 {
				name = string([]rune(name)[:9])
			}
			statLines = append(statLines, dim.Render(fmt.Sprintf("%-10s", name))+fg(p.Players[i%len(p.Players)]).Render(fmt.Sprintf("%-11s", bar))+text.Render(fmt.Sprint(l.wins)))
		}
	}
	statLines = append(statLines, "", accent.Render("AI BRAIN"),
		stat("Training games", m.learnedGames),
		stat("Learned positions", m.positions),
		stat("Decisions", m.decisions),
		stat("Search iterations", m.searches),
		dim.Render(fmt.Sprintf("%-18s", "Value network"))+fg(p.Accent).Render("● ONLINE"),
	)
	stats := panel.Width(36).Render(strings.Join(statLines, "\n"))
	body := lipgloss.JoinHorizontal(lipgloss.Top, setup, "  ", stats)

	key := lipgloss.NewStyle().Foreground(ansi(p.Background)).Background(ansi(p.Muted)).Padding(0, 1)
	help := strings.Join([]string{
		key.Render("↑↓") + dim.Render(" navigate"),
		key.Render("←→") + dim.Render(" change"),
		key.Render("S") + dim.Render(" start"),
		key.Render("W") + dim.Render(" watch AI"),
		key.Render("Q") + dim.Render(" quit"),
	}, "   ")
	msg := ""
	if m.message != "" {
		msg = "\n\n" + fg(p.Players[0]).Bold(true).Render("! "+m.message)
	}
	return lipgloss.NewStyle().Padding(1, 3).Render(strings.Join(logo, "\n") + "\n\n" + body + "\n\n" + help + msg)
}

type leader struct {
	name string
	wins int
}

// topWinners returns up to n players with the most wins, ties by name.
func topWinners(wins map[string]int, n int) []leader {
	var all []leader
	for name, w := range wins {
		if w > 0 {
			all = append(all, leader{name, w})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].wins != all[j].wins {
			return all[i].wins > all[j].wins
		}
		return all[i].name < all[j].name
	})
	if len(all) > n {
		all = all[:n]
	}
	return all
}

func average(total, count int) float64 {
	if count == 0 {
		return 0
	}
	return float64(total) / float64(count)
}
