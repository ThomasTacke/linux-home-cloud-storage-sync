package tui

import (
	"fmt"
	"path/filepath"
	"strings"

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

// newItem turns a picked path into a symlinkMap item, or explains why it can't be one.
func newItem(path, home, cloud string, existing []string) (string, error) {
	path = filepath.Clean(path)
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is not inside %s", short(path), short(home))
	}
	if path == cloud || strings.HasPrefix(path, cloud+string(filepath.Separator)) ||
		strings.HasPrefix(cloud, path+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is the cloud folder or contains it", short(path))
	}
	for _, e := range existing {
		switch {
		case e == rel:
			return "", fmt.Errorf("%s is already an entry", rel)
		case strings.HasPrefix(rel, e+"/"):
			return "", fmt.Errorf("%s is inside the entry %s", rel, e)
		case strings.HasPrefix(e, rel+"/"):
			return "", fmt.Errorf("%s contains the entry %s; adopting it would move that link too", rel, e)
		}
	}
	return filepath.ToSlash(rel), nil
}
