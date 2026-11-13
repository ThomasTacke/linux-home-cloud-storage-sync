package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// fixture builds a cloud and a home folder with one entry of every kind.
func fixture(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root) // paths in the details show as ~/home/..., ~/cloud/...
	cloud, home := filepath.Join(root, "cloud"), filepath.Join(root, "home")
	mkdir := func(p string) { must(t, os.MkdirAll(p, 0o755)) }

	mkdir(filepath.Join(cloud, ".config", "fish")) // linked
	mkdir(filepath.Join(home, ".config"))
	must(t, os.Symlink(filepath.Join(cloud, ".config", "fish"), filepath.Join(home, ".config", "fish")))
	mkdir(filepath.Join(cloud, "Pictures"))                                           // not linked yet
	must(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]"), 0o644)) // only in $HOME
	mkdir(filepath.Join(cloud, "Documents"))                                          // both
	mkdir(filepath.Join(home, "Documents"))
	mkdir(filepath.Join(cloud, "Music")) // broken link
	must(t, os.Symlink(filepath.Join(root, "gone"), filepath.Join(home, "Music")))
	// "Templates": nowhere

	return config.Config{
		CloudStorageHomeDir: cloud,
		LocalHomeDir:        home,
		SymlinkMap:          []string{".config/fish", "Pictures", ".gitconfig", "Documents", "Music", "Templates"},
	}
}

func press(m Model, keys ...tea.KeyPressMsg) Model {
	for _, k := range keys {
		next, _ := m.Update(k)
		m = next.(Model)
	}
	return m
}

func char(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

var (
	down  = tea.KeyPressMsg{Code: tea.KeyDown}
	esc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	enter = tea.KeyPressMsg{Code: tea.KeyEnter}
)

func screen(m Model) string { return ansi.Strip(m.View().Content) }

func sized(m Model, w, h int) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(Model)
}

func TestKinds(t *testing.T) {
	m := New(fixture(t), "", "")
	want := []kind{kindLinked, kindNotLinked, kindLocalOnly, kindBothExist, kindBadLink, kindNotInCloud}
	for i, e := range m.entries {
		if e.kind != want[i] {
			t.Errorf("%s: kind %d, want %d (status %v)", e.item, e.kind, want[i], e.status.State)
		}
	}
	if m.entries[2].what != "file" || m.entries[3].what != "folder" {
		t.Errorf("what = %q, %q", m.entries[2].what, m.entries[3].what)
	}
}

func TestScreenShowsEveryEntryAndTheSelectedDetails(t *testing.T) {
	m := sized(New(fixture(t), "", ""), 100, 30)
	out := screen(m)
	if testing.Verbose() {
		fmt.Println(out)
	}
	for _, want := range []string{
		"✓  .config/fish", "linked",
		"○  Pictures", "not linked yet", "→ link",
		"●  .gitconfig", "only in $HOME", "→ adopt",
		"●  Documents", "local folder + cloud", "→ force (backup)",
		"✗  Music", "broken link", "→ force",
		"·  Templates", "not in cloud yet",
		"✓ 1", "○ 1", "● 2", "✗ 1", // header summary
		"is a symlink to", "Key: d deletes the link", // details of the first entry
	} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q\n%s", want, out)
		}
	}
	if lines := strings.Count(out, "\n") + 1; lines > 30 {
		t.Errorf("screen has %d lines, window is 30", lines)
	}
}

func TestCursorMovesAndDetailsFollow(t *testing.T) {
	m := sized(New(fixture(t), "", ""), 100, 30)
	m = press(m, char('j'), down) // .gitconfig
	if e, _ := m.current(); e.item != ".gitconfig" {
		t.Fatalf("selected %q", e.item)
	}
	if out := screen(m); !strings.Contains(out, "Key: enter (or a) to adopt.") {
		t.Errorf("details lack the adopt key:\n%s", out)
	}
	m = press(m, char('G'))
	if e, _ := m.current(); e.item != "Templates" {
		t.Errorf("G selected %q", e.item)
	}
	m = press(m, char('j'), char('j')) // stays at the end
	if e, _ := m.current(); e.item != "Templates" {
		t.Errorf("past the end selected %q", e.item)
	}
	m = press(m, char('g'))
	if e, _ := m.current(); e.item != ".config/fish" {
		t.Errorf("g selected %q", e.item)
	}
}

func TestFilter(t *testing.T) {
	m := sized(New(fixture(t), "", ""), 100, 30)
	m = press(m, char('/'), char('m'), char('u'))
	if m.mode != modeFilter || len(m.visible) != 1 {
		t.Fatalf("filtering=%v visible=%d", m.mode == modeFilter, len(m.visible))
	}
	// "q" is typed into the filter, not quitting
	m = press(m, char('q'))
	if m.filter.Value() != "muq" || len(m.visible) != 0 {
		t.Fatalf("filter %q, visible %d", m.filter.Value(), len(m.visible))
	}
	if out := screen(m); !strings.Contains(out, "No entry matches the filter.") {
		t.Errorf("no empty-filter notice:\n%s", out)
	}
	m = press(m, tea.KeyPressMsg{Code: tea.KeyBackspace}, enter)
	if m.mode == modeFilter || len(m.visible) != 1 {
		t.Fatalf("after enter: filtering=%v visible=%d", m.mode == modeFilter, len(m.visible))
	}
	if e, _ := m.current(); e.item != "Music" {
		t.Errorf("selected %q", e.item)
	}
	m = press(m, esc)
	if m.filter.Value() != "" || len(m.visible) != 6 {
		t.Errorf("esc: filter %q, visible %d", m.filter.Value(), len(m.visible))
	}
}

func TestScrollsLongLists(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{CloudStorageHomeDir: filepath.Join(root, "c"), LocalHomeDir: filepath.Join(root, "h")}
	for i := range 40 {
		cfg.SymlinkMap = append(cfg.SymlinkMap, fmt.Sprintf("entry-%02d", i))
	}
	m := sized(New(cfg, "", ""), 80, 20)
	for range 25 {
		m = press(m, char('j'))
	}
	out := screen(m)
	if !strings.Contains(out, "▸   ·  entry-25") {
		t.Errorf("cursor row not visible:\n%s", out)
	}
	if strings.Contains(out, "entry-00") {
		t.Errorf("first entry still shown after scrolling:\n%s", out)
	}
	if lines := strings.Count(out, "\n") + 1; lines > 20 {
		t.Errorf("screen has %d lines, window is 20", lines)
	}
}

func TestRefreshKeepsSelectionAndSeesChanges(t *testing.T) {
	cfg := fixture(t)
	m := sized(New(cfg, "", ""), 100, 30)
	m = press(m, char('j')) // Pictures: not linked yet
	must(t, os.Symlink(filepath.Join(cfg.CloudStorageHomeDir, "Pictures"), filepath.Join(cfg.LocalHomeDir, "Pictures")))

	m = press(m, char('r'))

	e, _ := m.current()
	if e.item != "Pictures" || e.kind != kindLinked {
		t.Errorf("after refresh: %q kind %d", e.item, e.kind)
	}
}

func TestQuit(t *testing.T) {
	m := New(fixture(t), "", "")
	_, cmd := m.Update(char('q'))
	if cmd == nil {
		t.Fatal("q returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q doesn't quit")
	}
}

// The TUI is read-only: looking at everything must not change the file system.
func TestChangesNothing(t *testing.T) {
	cfg := fixture(t)
	before := snapshot(t, filepath.Dir(cfg.LocalHomeDir))
	m := sized(New(cfg, "", ""), 100, 30)
	m = press(m, char('G'), char('r'), char('g'), char('?'))
	_ = screen(m)
	if after := snapshot(t, filepath.Dir(cfg.LocalHomeDir)); after != before {
		t.Errorf("file system changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	must(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest, _ := os.Readlink(path)
		fmt.Fprintf(&b, "%s %v %s\n", path, d.Type(), dest)
		return nil
	}))
	return b.String()
}
