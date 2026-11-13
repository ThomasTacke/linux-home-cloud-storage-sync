package tui

import (
	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
)

// newPicker is the "new entry" file browser, starting in $HOME with dotfiles shown (most
// entries are dotfiles) and files and folders selectable.
func newPicker(home string, height int) filepicker.Model {
	p := filepicker.New()
	p.CurrentDirectory = home
	p.ShowHidden = true
	p.DirAllowed = true
	p.FileAllowed = true
	p.ShowSize = false
	p.ShowPermissions = false
	p.Cursor = "▸"
	p.Styles.Cursor = cursorStyle
	p.Styles.Selected = cursorStyle
	p.Styles.Directory = p.Styles.Directory.Foreground(green)
	p.Styles.Symlink = p.Styles.Symlink.Foreground(orange)
	// esc closes the picker instead of going up a folder; ←, h and backspace still do
	p.KeyMap.Back = key.NewBinding(key.WithKeys("h", "backspace", "left"), key.WithHelp("←", "up"))
	// → and l open a folder; enter selects (a folder too)
	p.KeyMap.Open = key.NewBinding(key.WithKeys("l", "right", "enter"), key.WithHelp("→", "open folder"))
	p.SetHeight(max(height, 3))
	return p
}
