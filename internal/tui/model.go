// Package tui is the interactive view of the config: one line per symlinkMap entry with
// its status, details for the selected one, a filter and a help bar. Step 1 is
// read-only: it only runs dry runs and shows the command that would fix an entry.
//
// It follows Bubble Tea's Elm architecture:
//
//   - Model holds the whole state (entries, cursor, filter, window size).
//   - Update gets every event (key, window resize) as a message and returns the new model.
//     It never draws.
//   - View turns the model into the screen. It never changes anything.
//
// Bubble Tea calls Update for each message and View after each Update.
package tui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
)

// Model is the TUI's state. Bubble Tea passes it by value: Update returns a changed copy.
type Model struct {
	cfg  config.Config
	note string // shown in the header, e.g. "defaults, no config found"

	entries []entry
	visible []int // indexes into entries that match the filter
	cursor  int   // position in visible
	offset  int   // first visible row when the list is longer than the screen

	filter    textinput.Model
	filtering bool // typing into the filter, keys go to the text input

	keys keyMap
	help help.Model

	width, height int
}

// New builds the model for a config. note is shown in the header.
func New(cfg config.Config, note string) Model {
	filter := textinput.New()
	filter.Prompt = "/ "
	filter.Placeholder = "filter entries"

	m := Model{
		cfg:    cfg,
		note:   note,
		filter: filter,
		keys:   newKeyMap(),
		help:   help.New(),
		width:  80,
		height: 24,
	}
	m.refresh()
	return m
}

// Init runs once at start. No background work needed: inspecting is quick and done in New.
func (m Model) Init() tea.Cmd {
	return nil
}

// refresh checks all entries again (dry runs) and keeps the cursor on the same entry.
func (m *Model) refresh() {
	selected := ""
	if e, ok := m.selected(); ok {
		selected = e.item
	}
	m.entries = inspect(m.cfg)
	m.applyFilter()
	for i, idx := range m.visible {
		if m.entries[idx].item == selected {
			m.cursor = i
		}
	}
	m.clamp()
}

// applyFilter keeps the entries whose item contains the filter text (case-insensitive).
func (m *Model) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	m.visible = m.visible[:0]
	for i, e := range m.entries {
		if query == "" || strings.Contains(strings.ToLower(e.item), query) {
			m.visible = append(m.visible, i)
		}
	}
	m.clamp()
}

func (m Model) selected() (entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return entry{}, false
	}
	return m.entries[m.visible[m.cursor]], true
}

// clamp keeps cursor inside the list and the cursor row inside the visible window.
func (m *Model) clamp() {
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	rows := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if maxOffset := len(m.visible) - rows; m.offset > maxOffset {
		m.offset = max(maxOffset, 0)
	}
}

// Update handles one message and returns the new model, plus an optional command
// (tea.Quit ends the program).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
		m.clamp()
		return m, nil

	case tea.KeyPressMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			m.cursor--
		case key.Matches(msg, m.keys.Down):
			m.cursor++
		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
		case key.Matches(msg, m.keys.Bottom):
			m.cursor = len(m.visible) - 1
		case key.Matches(msg, m.keys.Refresh):
			m.refresh()
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Cancel):
			m.filter.Reset()
			m.applyFilter()
		case key.Matches(msg, m.keys.Filter):
			m.filtering = true
			return m, m.filter.Focus() // Focus returns the cursor-blink command
		}
		m.clamp()
	}
	return m, nil
}

// updateFilter: while filtering, keys go to the text input, except enter and esc.
func (m Model) updateFilter(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Apply):
		m.filtering = false
		m.filter.Blur()
		return m, nil
	case key.Matches(msg, m.keys.Cancel):
		m.filtering = false
		m.filter.Blur()
		m.filter.Reset()
		m.applyFilter()
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.cursor = 0
	m.applyFilter()
	return m, cmd
}

// View draws the screen from the model; see view.go.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true // full screen; the terminal is restored on quit
	v.WindowTitle = "linux-home-cloud-storage-sync"
	return v
}
