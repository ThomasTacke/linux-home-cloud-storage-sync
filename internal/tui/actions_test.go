package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

// run sends keys and also runs the commands they return, feeding each resulting message
// back into Update, like Bubble Tea does. Blink and other timer commands are skipped.
func run(t *testing.T, m Model, keys ...tea.KeyPressMsg) Model {
	t.Helper()
	for _, k := range keys {
		next, cmd := m.Update(k)
		m = settle(t, next.(Model), cmd)
	}
	return m
}

func settle(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for i := 0; cmd != nil && i < 10; i++ {
		if m.mode == modeFilter {
			return m // the text input's cursor blink waits forever
		}
		msg := cmd()
		if msg == nil {
			return m
		}
		next, nextCmd := m.Update(msg)
		m, cmd = next.(Model), nextCmd
	}
	return m
}

// withConfigFile writes cfg to a config file and returns the model and the file's path.
func withConfigFile(t *testing.T, cfg config.Config) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	must(t, cfg.Write(path))
	return sized(New(cfg, path, ""), 110, 34), path
}

func savedItems(t *testing.T, path string) []string {
	t.Helper()
	cfg, err := config.Load(path)
	must(t, err)
	return cfg.SymlinkMap
}

func space() tea.KeyPressMsg { return tea.KeyPressMsg{Code: ' ', Text: " "} }

func TestSpaceSelectsAndMovesOn(t *testing.T) {
	m := sized(New(fixture(t), "", ""), 100, 30)
	m = press(m, space(), space())
	if !m.selected[".config/fish"] || !m.selected["Pictures"] || len(m.selected) != 2 {
		t.Fatalf("selected = %v", m.selected)
	}
	if e, _ := m.current(); e.item != ".gitconfig" {
		t.Errorf("cursor on %q", e.item)
	}
	if out := screen(m); strings.Count(out, "◆") != 2 {
		t.Errorf("not two selection markers:\n%s", out)
	}
	m = press(m, char('k'), space()) // unselect Pictures
	if m.selected["Pictures"] {
		t.Errorf("space didn't unselect")
	}
}

func TestSelectAllAndEscClearsSelectionThenFilter(t *testing.T) {
	m := sized(New(fixture(t), "", ""), 100, 30)
	m = press(m, char('/'), char('i'), char('c'), enter) // Music, Pictures
	m = press(m, tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	if len(m.selected) != 2 || !m.selected["Music"] || !m.selected["Pictures"] {
		t.Fatalf("ctrl+a selected %v", m.selected)
	}
	m = press(m, esc)
	if len(m.selected) != 0 || m.filter.Value() != "ic" {
		t.Fatalf("first esc: selected %v, filter %q", m.selected, m.filter.Value())
	}
	m = press(m, esc)
	if m.filter.Value() != "" || len(m.visible) != 6 {
		t.Errorf("second esc: filter %q, visible %d", m.filter.Value(), len(m.visible))
	}
}

func TestFixSelectedEntries(t *testing.T) {
	cfg := fixture(t)
	m, _ := withConfigFile(t, cfg)
	// select everything except Templates (nothing to do there anyway)
	m = press(m, tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})

	m = run(t, m, enter)

	if m.mode != modeConfirm {
		t.Fatalf("mode %d, want confirm; status %q", m.mode, m.status)
	}
	if m.plan.count() != 4 {
		t.Fatalf("plan does %d, want 4 (link, adopt, force backup, force link)", m.plan.count())
	}
	out := screen(m)
	for _, want := range []string{"Fix 4 entries?", "2 have nothing to do and are skipped",
		"Pictures", "link to ~/cloud/Pictures", ".gitconfig", "move to ~/cloud/.gitconfig, link it back",
		"Documents", "rename to Documents.backup-", "Music", "replace the link to ~/gone",
		"skip: already linked", "skip: not in the cloud", "y/enter do it"} {
		if !strings.Contains(out, want) {
			t.Errorf("dialog lacks %q\n%s", want, out)
		}
	}

	m = run(t, m, char('y'))

	if m.mode != modeResult || m.status != "Fix: 4 done." {
		t.Fatalf("mode %d, status %q\n%s", m.mode, m.status, screen(m))
	}
	for _, item := range []string{"Pictures", ".gitconfig", "Documents", "Music"} {
		if got := m.stateOf(item); got != linker.AlreadyLinked {
			t.Errorf("%s: state %v after fix", item, got)
		}
	}
	if backups, _ := filepath.Glob(filepath.Join(cfg.LocalHomeDir, "Documents.backup-*")); len(backups) != 1 {
		t.Errorf("backups = %v", backups)
	}
	if len(m.selected) != 0 {
		t.Errorf("selection kept after success: %v", m.selected)
	}
	m = press(m, char('z')) // any key closes the result
	if m.mode != modeList {
		t.Errorf("result not closed, mode %d", m.mode)
	}
}

func TestCancelChangesNothing(t *testing.T) {
	cfg := fixture(t)
	before := snapshot(t, filepath.Dir(cfg.LocalHomeDir))
	m, _ := withConfigFile(t, cfg)
	m = press(m, char('j'), char('j'), char('a')) // .gitconfig: adopt
	if m.mode != modeConfirm {
		t.Fatalf("no dialog, status %q", m.status)
	}
	m = run(t, m, esc)
	if m.mode != modeList {
		t.Fatalf("mode %d after esc", m.mode)
	}
	m = run(t, m, char('a'), char('n'))
	if after := snapshot(t, filepath.Dir(cfg.LocalHomeDir)); after != before {
		t.Errorf("file system changed after cancelling")
	}
}

func TestNothingToDoShowsStatusNoDialog(t *testing.T) {
	m := sized(New(fixture(t), "", ""), 100, 30)
	m = press(m, char('l')) // .config/fish is linked already
	if m.mode != modeList || !strings.Contains(m.status, "Link: nothing to do for .config/fish.") {
		t.Errorf("mode %d, status %q", m.mode, m.status)
	}
	if !strings.Contains(screen(m), "Link: nothing to do") {
		t.Errorf("status not shown")
	}
}

func TestDeleteLinkAndRestore(t *testing.T) {
	cfg := fixture(t)
	m, _ := withConfigFile(t, cfg)

	m = run(t, m, char('D'), char('y')) // .config/fish: replace the link with a copy
	if info, err := os.Lstat(filepath.Join(cfg.LocalHomeDir, ".config", "fish")); err != nil || !info.IsDir() {
		t.Fatalf("not restored as folder: %v %v\n%s", info, err, screen(m))
	}
	if _, err := os.Stat(filepath.Join(cfg.CloudStorageHomeDir, ".config", "fish")); err != nil {
		t.Errorf("cloud copy gone")
	}

	must(t, os.RemoveAll(filepath.Join(cfg.LocalHomeDir, ".config", "fish")))
	must(t, os.Symlink(filepath.Join(cfg.CloudStorageHomeDir, ".config", "fish"), filepath.Join(cfg.LocalHomeDir, ".config", "fish")))
	m = run(t, m, char('z'), char('r'), char('d'), char('y'))
	if _, err := os.Lstat(filepath.Join(cfg.LocalHomeDir, ".config", "fish")); !os.IsNotExist(err) {
		t.Errorf("link still there: %v\n%s", err, screen(m))
	}
}

func TestRemoveFromConfigKeepsFiles(t *testing.T) {
	cfg := fixture(t)
	before := snapshot(t, filepath.Dir(cfg.LocalHomeDir))
	m, path := withConfigFile(t, cfg)
	m = press(m, char('j'), space(), space()) // Pictures, .gitconfig

	m = run(t, m, char('x'))
	if out := screen(m); !strings.Contains(out, "Remove from config 2 entries?") {
		t.Fatalf("no dialog:\n%s", out)
	}
	m = run(t, m, char('y'))

	want := []string{".config/fish", "Documents", "Music", "Templates"}
	if got := savedItems(t, path); !slices.Equal(got, want) {
		t.Errorf("saved %v, want %v", got, want)
	}
	if len(m.entries) != 4 {
		t.Errorf("list has %d entries", len(m.entries))
	}
	if after := snapshot(t, filepath.Dir(cfg.LocalHomeDir)); after != before {
		t.Errorf("removing from the config changed files")
	}
}

func TestNewEntryThroughThePicker(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	cfg := config.Config{
		CloudStorageHomeDir: filepath.Join(root, "Nextcloud"),
		LocalHomeDir:        filepath.Join(root, "home"),
		SymlinkMap:          []string{},
	}
	must(t, os.MkdirAll(cfg.CloudStorageHomeDir, 0o755))
	must(t, os.MkdirAll(cfg.LocalHomeDir, 0o755))
	must(t, os.WriteFile(filepath.Join(cfg.LocalHomeDir, "notes.txt"), []byte("hi"), 0o644))
	m, path := withConfigFile(t, cfg)

	m = run(t, m, char('n'))
	if m.mode != modePicker || !strings.Contains(screen(m), "notes.txt") {
		t.Fatalf("picker not open or empty, mode %d:\n%s", m.mode, screen(m))
	}
	m = run(t, m, enter) // the only file, highlighted

	// Added, saved, and the adopt dialog opened right away
	if got := savedItems(t, path); !slices.Equal(got, []string{"notes.txt"}) {
		t.Fatalf("saved %v; status %q", got, m.status)
	}
	if m.mode != modeConfirm || m.plan.action != actFix {
		t.Fatalf("no fix dialog, mode %d, status %q", m.mode, m.status)
	}
	m = run(t, m, char('y'))
	if m.stateOf("notes.txt") != linker.AlreadyLinked {
		t.Errorf("not adopted:\n%s", screen(m))
	}
	if data, _ := os.ReadFile(filepath.Join(cfg.CloudStorageHomeDir, "notes.txt")); string(data) != "hi" {
		t.Errorf("cloud copy = %q", data)
	}
}

func TestPickerEscCancels(t *testing.T) {
	m, path := withConfigFile(t, fixture(t))
	m = run(t, m, char('n'), esc)
	if m.mode != modeList {
		t.Errorf("mode %d after esc", m.mode)
	}
	if got := savedItems(t, path); len(got) != 6 {
		t.Errorf("config changed: %v", got)
	}
}

func TestNewItemChecks(t *testing.T) {
	home, cloud := "/home/kiwi", "/home/kiwi/Nextcloud/LinuxHome"
	existing := []string{".config/fish", "Documents"}
	tests := []struct {
		path, want, err string
	}{
		{"/home/kiwi/.gitconfig", ".gitconfig", ""},
		{"/home/kiwi/.config/nvim", ".config/nvim", ""},
		{"/home/kiwi", "", "not inside"},
		{"/etc/hosts", "", "not inside"},
		{"/home/kiwi/Nextcloud", "", "cloud folder"},
		{"/home/kiwi/Nextcloud/LinuxHome/x", "", "cloud folder"},
		{"/home/kiwi/Documents", "", "already an entry"},
		{"/home/kiwi/Documents/cv.pdf", "", "inside the entry Documents"},
		{"/home/kiwi/.config", "", "contains the entry .config/fish"},
	}
	for _, tt := range tests {
		got, err := newItem(tt.path, home, cloud, existing)
		if tt.err == "" && (err != nil || got != tt.want) {
			t.Errorf("%s: got %q, %v; want %q", tt.path, got, err, tt.want)
		}
		if tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)) {
			t.Errorf("%s: err %v, want %q", tt.path, err, tt.err)
		}
	}
}

func TestBusyIgnoresKeys(t *testing.T) {
	m, _ := withConfigFile(t, fixture(t))
	m = press(m, char('j'), char('l')) // Pictures: link
	next, cmd := m.Update(char('y'))   // don't run the command yet
	m = next.(Model)
	if m.mode != modeBusy || cmd == nil {
		t.Fatalf("mode %d, cmd %v", m.mode, cmd != nil)
	}
	m = press(m, char('q'), char('y'), esc)
	if m.mode != modeBusy {
		t.Errorf("keys changed the busy mode to %d", m.mode)
	}
	next, _ = m.Update(cmd())
	if m = next.(Model); m.mode != modeResult {
		t.Errorf("mode %d after done", m.mode)
	}
}

// The config file keeps "~" and unknown keys when the TUI saves it.
func TestSavingKeepsTheConfigsOwnPaths(t *testing.T) {
	cfg := fixture(t)
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{"cloudStorageHomeDir": "~/cloud", "localHomeDir": "~/home", "symlinkMap": ` +
		`[".config/fish", "Pictures", ".gitconfig", "Documents", "Music", "Templates"]}`
	must(t, os.WriteFile(path, []byte(raw), 0o644))
	m := sized(New(cfg, path, ""), 110, 34)

	m = run(t, m, char('G'), char('x'), char('y')) // remove Templates

	var saved map[string]any
	data, _ := os.ReadFile(path)
	must(t, json.Unmarshal(data, &saved))
	if saved["cloudStorageHomeDir"] != "~/cloud" || saved["localHomeDir"] != "~/home" {
		t.Errorf("paths rewritten:\n%s", data)
	}
	_ = m
}
