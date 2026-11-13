package tui

import "charm.land/bubbles/v2/key"

// keyMap lists every key the TUI reacts to. key.Binding keeps the keys and their help text
// together, and the help bar at the bottom is generated from ShortHelp and FullHelp.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Top     key.Binding
	Bottom  key.Binding
	Filter  key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
	// Only while typing a filter
	Apply  key.Binding
	Cancel key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Top:     key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
		Bottom:  key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
		Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more keys")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Apply:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "keep filter")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")),
	}
}

// ShortHelp is the one-line help bar (help.KeyMap interface).
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Filter, k.Refresh, k.Help, k.Quit}
}

// FullHelp is shown after "?", in columns.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom},
		{k.Filter, k.Cancel, k.Refresh},
		{k.Help, k.Quit},
	}
}

// filterKeys is the help bar while typing a filter.
type filterKeys struct{ k keyMap }

func (f filterKeys) ShortHelp() []key.Binding  { return []key.Binding{f.k.Apply, f.k.Cancel} }
func (f filterKeys) FullHelp() [][]key.Binding { return [][]key.Binding{f.ShortHelp()} }
