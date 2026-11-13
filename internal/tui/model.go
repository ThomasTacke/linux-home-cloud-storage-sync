// Package tui is the interactive view of the config: one line per symlinkMap entry with
// its status, details for the selected one, a filter and a help bar, plus actions on the
// selected entries (always previewed as a dry run first) and adding new entries.
//
// It follows Bubble Tea's Elm architecture:
//
//   - Model holds the whole state (entries, cursor, filter, dialogs, window size).
//   - Update gets every event (key, window resize, finished work) as a message and returns
//     the new model. It never draws.
//   - View turns the model into the screen. It never changes anything.
//
// Bubble Tea calls Update for each message and View after each Update. Slow work (moving
// files) runs as a tea.Cmd in the background and reports back with a message.
package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/app"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

// mode is which part of the screen gets the keys.
type mode int

const (
	modeList    mode = iota // moving, selecting, starting actions
	modeFilter              // typing into the filter
	modeConfirm             // dialog: preview of an action, yes or no
	modeBusy                // the action runs, keys wait
	modeResult              // dialog: what the action did, any key closes it
	modePicker              // dialog: choose a file or folder for a new entry
)

// Model is the TUI's state. Bubble Tea passes it by value: Update returns a changed copy.
type Model struct {
	cfg        config.Config
	configPath string
	note       string // shown in the header, e.g. "defaults, no config found"

	entries  []entry
	visible  []int           // indexes into entries that match the filter
	cursor   int             // position in visible
	offset   int             // first visible row when the list is longer than the screen
	selected map[string]bool // marked with space, by item

	mode   mode
	filter textinput.Model
	plan   plan    // modeConfirm: what would happen
	done   doneMsg // modeResult: what happened
	picker filepicker.Model
	status string // last message in the footer, e.g. "3 linked"

	keys keyMap
	help help.Model

	width, height int
}

// New builds the model for a config loaded from configPath (which may not exist yet; it's
// created when an entry is added). note is shown in the header.
func New(cfg config.Config, configPath, note string) Model {
	filter := textinput.New()
	filter.Prompt = "/ "
	filter.Placeholder = "filter entries"

	m := Model{
		cfg:        cfg,
		configPath: configPath,
		note:       note,
		selected:   map[string]bool{},
		filter:     filter,
		keys:       newKeyMap(),
		help:       help.New(),
		width:      80,
		height:     24,
	}
	m.refresh()
	return m
}

// Init runs once at start. No background work needed: inspecting is quick and done in New.
func (m Model) Init() tea.Cmd {
	return nil
}

// refresh checks all entries again (dry runs) and keeps the cursor on the same entry.
// Selections of entries that are gone are dropped.
func (m *Model) refresh() {
	current := ""
	if e, ok := m.current(); ok {
		current = e.Item
	}
	m.entries = app.Inspect(m.cfg)
	for item := range m.selected {
		if !slices.Contains(m.cfg.SymlinkMap, item) {
			delete(m.selected, item)
		}
	}
	m.applyFilter()
	m.moveTo(current)
}

// moveTo puts the cursor on item, if it's visible.
func (m *Model) moveTo(item string) {
	for i, idx := range m.visible {
		if m.entries[idx].Item == item {
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
		if query == "" || strings.Contains(strings.ToLower(e.Item), query) {
			m.visible = append(m.visible, i)
		}
	}
	m.clamp()
}

// current is the entry under the cursor.
func (m Model) current() (entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return entry{}, false
	}
	return m.entries[m.visible[m.cursor]], true
}

// targets are the entries an action applies to: the selected ones, or else the one under
// the cursor.
func (m Model) targets() []entry {
	var out []entry
	for _, e := range m.entries {
		if m.selected[e.Item] {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		if e, ok := m.current(); ok {
			out = append(out, e)
		}
	}
	return out
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

// Update handles one message and returns the new model, plus an optional command: work
// for Bubble Tea to do in the background (tea.Quit ends the program).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
		m.picker.SetHeight(m.pickerHeight())
		m.clamp()
		return m, nil

	case doneMsg:
		return m.finished(msg), nil

	case tea.KeyPressMsg:
		switch m.mode {
		case modeFilter:
			return m.updateFilter(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeBusy:
			return m, nil
		case modeResult:
			m.mode = modeList // any key closes the result
			return m, nil
		case modePicker:
			return m.updatePicker(msg)
		}
		return m.updateList(msg)
	}

	// Anything else (e.g. the file picker's directory listing) goes to the picker
	if m.mode == modePicker {
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.status = ""
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
	case key.Matches(msg, m.keys.Select):
		if e, ok := m.current(); ok {
			if m.selected[e.Item] {
				delete(m.selected, e.Item)
			} else {
				m.selected[e.Item] = true
			}
			m.cursor++ // like a file manager: mark and move on
		}
	case key.Matches(msg, m.keys.All):
		for _, idx := range m.visible {
			m.selected[m.entries[idx].Item] = true
		}
	case key.Matches(msg, m.keys.Clear):
		if len(m.selected) > 0 {
			clear(m.selected)
		} else {
			m.filter.Reset()
			m.applyFilter()
		}
	case key.Matches(msg, m.keys.Refresh):
		m.refresh()
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
	case key.Matches(msg, m.keys.Filter):
		m.mode = modeFilter
		return m, m.filter.Focus() // Focus returns the cursor-blink command
	case key.Matches(msg, m.keys.New):
		m.mode = modePicker
		m.picker = newPicker(m.cfg.LocalHomeDir, m.pickerHeight())
		return m, m.picker.Init() // reads the directory in the background
	case key.Matches(msg, m.keys.Fix):
		return m.confirm(actFix), nil
	case key.Matches(msg, m.keys.Link):
		return m.confirm(actLink), nil
	case key.Matches(msg, m.keys.Adopt):
		return m.confirm(actAdopt), nil
	case key.Matches(msg, m.keys.Force):
		return m.confirm(actForce), nil
	case key.Matches(msg, m.keys.Unlink):
		return m.confirm(actUnlink), nil
	case key.Matches(msg, m.keys.Restore):
		return m.confirm(actRestore), nil
	case key.Matches(msg, m.keys.Remove):
		return m.confirm(actRemove), nil
	}
	m.clamp()
	return m, nil
}

// confirm previews an action for the targets and opens the dialog, unless there's
// nothing to do.
func (m Model) confirm(a action) Model {
	targets := m.targets()
	if len(targets) == 0 {
		return m
	}
	p := app.Preview(a, targets)
	if p.Count() == 0 {
		m.status = fmt.Sprintf("%s: nothing to do for %s.", actionTitle(a), describeTargets(targets))
		return m
	}
	m.plan = p
	m.mode = modeConfirm
	return m
}

func describeTargets(targets []entry) string {
	if len(targets) == 1 {
		return targets[0].Item
	}
	return fmt.Sprintf("the %d selected entries", len(targets))
}

func (m Model) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Yes):
		m.mode = modeBusy
		return m, execute(m.plan, m.entries, m.cfg, m.configPath)
	case key.Matches(msg, m.keys.No):
		m.mode = modeList
	}
	return m, nil
}

// finished shows what the action did and checks every entry again.
func (m Model) finished(done doneMsg) Model {
	m.done = done
	m.mode = modeResult
	m.cfg = done.Config // ActRemove changed symlinkMap
	failed := app.Result(done).Failed()
	switch {
	case done.Err != nil:
		m.status = "Saving the config failed: " + done.Err.Error()
	case failed > 0:
		m.status = fmt.Sprintf("%s: %d of %d failed.", actionTitle(done.Action), failed, len(done.Steps))
	default:
		m.status = fmt.Sprintf("%s: %d done.", actionTitle(done.Action), len(done.Steps))
		clear(m.selected)
	}
	m.refresh()
	return m
}

func (m Model) updatePicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" || msg.String() == "q" {
		m.mode = modeList
		return m, nil
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	if ok, path := m.picker.DidSelectFile(msg); ok {
		return m.add(path), nil
	}
	return m, cmd
}

// add puts the picked path into the config, saves it and offers the fitting action right
// away: adopt for something only in $HOME, link when the cloud already has it.
func (m Model) add(path string) Model {
	m.mode = modeList
	cfg, item, err := app.AddItem(m.cfg, m.configPath, path)
	if err != nil {
		m.status = "Not added: " + err.Error()
		return m
	}
	m.cfg = cfg
	m.note = "" // a config exists now
	m.filter.Reset()
	clear(m.selected)
	m.refresh()
	m.moveTo(item)
	m.status = fmt.Sprintf("Added %s to %s.", item, short(m.configPath))

	if e, ok := m.current(); ok && (e.Kind == kindLocalOnly || e.Kind == kindNotLinked) {
		status := m.status
		m = m.confirm(actFix)
		if m.mode != modeConfirm {
			m.status = status
		}
	}
	return m
}

// updateFilter: while filtering, keys go to the text input, except enter and esc.
func (m Model) updateFilter(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Apply):
		m.mode = modeList
		m.filter.Blur()
		return m, nil
	case key.Matches(msg, m.keys.Cancel):
		m.mode = modeList
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

// stateOf is a helper for tests and the view: the linker state of an item right now.
func (m Model) stateOf(item string) linker.State {
	for _, e := range m.entries {
		if e.Item == item {
			return e.Status.State
		}
	}
	return -1
}
