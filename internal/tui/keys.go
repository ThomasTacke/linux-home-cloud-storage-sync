package tui

import "charm.land/bubbles/v2/key"

// keyMap lists every key the TUI reacts to. key.Binding keeps the keys and their help text
// together, and the help bar at the bottom is generated from ShortHelp and FullHelp.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Top     key.Binding
	Bottom  key.Binding
	Select  key.Binding
	All     key.Binding
	Fix     key.Binding
	Link    key.Binding
	Adopt   key.Binding
	Force   key.Binding
	Unlink  key.Binding
	Restore key.Binding
	New     key.Binding
	Remove  key.Binding
	Filter  key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
	// esc in the list: clears the selection, then the filter
	Clear key.Binding
	// Only while typing a filter
	Apply  key.Binding
	Cancel key.Binding
	// In dialogs
	Yes key.Binding
	No  key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Top:     key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
		Bottom:  key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
		Select:  key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select")),
		All:     key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "select all")),
		Fix:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "fix")),
		Link:    key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "link")),
		Adopt:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "adopt")),
		Force:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "force")),
		Unlink:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete link")),
		Restore: key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete link, keep copy")),
		New:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new entry")),
		Remove:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "remove from config")),
		Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more keys")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Clear:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear selection/filter")),
		Apply:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "keep filter")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")),
		Yes:     key.NewBinding(key.WithKeys("y", "enter"), key.WithHelp("y/enter", "do it")),
		No:      key.NewBinding(key.WithKeys("n", "esc", "q"), key.WithHelp("n/esc", "cancel")),
	}
}

// ShortHelp is the one-line help bar (help.KeyMap interface).
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Fix, k.New, k.Filter, k.Help, k.Quit}
}

// FullHelp is shown after "?", in columns.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom},
		{k.Select, k.All, k.Clear, k.Filter},
		{k.Fix, k.Link, k.Adopt, k.Force},
		{k.Unlink, k.Restore, k.New, k.Remove},
		{k.Refresh, k.Help, k.Quit},
	}
}

// filterKeys is the help bar while typing a filter.
type filterKeys struct{ k keyMap }

func (f filterKeys) ShortHelp() []key.Binding  { return []key.Binding{f.k.Apply, f.k.Cancel} }
func (f filterKeys) FullHelp() [][]key.Binding { return [][]key.Binding{f.ShortHelp()} }

// confirmKeys is the help bar of the confirmation dialog.
type confirmKeys struct{ k keyMap }

func (c confirmKeys) ShortHelp() []key.Binding  { return []key.Binding{c.k.Yes, c.k.No} }
func (c confirmKeys) FullHelp() [][]key.Binding { return [][]key.Binding{c.ShortHelp()} }

// staticKeys is a help bar from fixed bindings (results, file picker).
type staticKeys []key.Binding

func (s staticKeys) ShortHelp() []key.Binding  { return s }
func (s staticKeys) FullHelp() [][]key.Binding { return [][]key.Binding{s} }
