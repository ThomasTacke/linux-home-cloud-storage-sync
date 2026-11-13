package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// fixture: one entry of every kind, the config saved to a file.
func fixture(t *testing.T) (config.Config, string) {
	t.Helper()
	root := t.TempDir()
	cloud, home := filepath.Join(root, "cloud"), filepath.Join(root, "home")
	mkdir := func(p string) { must(t, os.MkdirAll(p, 0o755)) }
	mkdir(filepath.Join(cloud, "Linked"))
	mkdir(home)
	must(t, os.Symlink(filepath.Join(cloud, "Linked"), filepath.Join(home, "Linked")))
	mkdir(filepath.Join(cloud, "CloudOnly"))
	must(t, os.WriteFile(filepath.Join(home, "local.txt"), []byte("x"), 0o644))
	mkdir(filepath.Join(cloud, "Both"))
	mkdir(filepath.Join(home, "Both"))
	mkdir(filepath.Join(cloud, "Broken"))
	must(t, os.Symlink(filepath.Join(root, "gone"), filepath.Join(home, "Broken")))
	cfg := config.Config{
		CloudStorageHomeDir: cloud,
		LocalHomeDir:        home,
		SymlinkMap:          []string{"Linked", "CloudOnly", "local.txt", "Both", "Broken", "Nowhere"},
	}
	path := filepath.Join(root, "config.json")
	must(t, cfg.Write(path))
	return cfg, path
}

func TestInspectKinds(t *testing.T) {
	cfg, _ := fixture(t)
	want := []Kind{KindLinked, KindNotLinked, KindLocalOnly, KindBothExist, KindBadLink, KindNotInCloud}
	for i, e := range Inspect(cfg) {
		if e.Kind != want[i] {
			t.Errorf("%s: kind %d, want %d", e.Item, e.Kind, want[i])
		}
	}
}

func TestPreviewChangesNothingAndApplyDoes(t *testing.T) {
	cfg, path := fixture(t)
	entries := Inspect(cfg)

	p := Preview(ActFix, entries)
	if p.Count() != 4 {
		t.Fatalf("fix would change %d entries, want 4", p.Count())
	}
	if again := Inspect(cfg); !slices.EqualFunc(again, entries, func(a, b Entry) bool { return a.Kind == b.Kind }) {
		t.Fatalf("preview changed something")
	}

	res := Apply(p, entries, cfg, path)

	if len(res.Steps) != 4 || res.Failed() != 0 {
		t.Fatalf("steps %d, failed %d", len(res.Steps), res.Failed())
	}
	for _, e := range Inspect(cfg) {
		if e.Item != "Nowhere" && e.Kind != KindLinked {
			t.Errorf("%s: kind %d after fix", e.Item, e.Kind)
		}
	}
}

func TestApplyRemoveSavesTheConfig(t *testing.T) {
	cfg, path := fixture(t)
	entries := Inspect(cfg)
	p := Preview(ActRemove, entries[:2])

	res := Apply(p, entries, cfg, path)

	want := []string{"local.txt", "Both", "Broken", "Nowhere"}
	if res.Err != nil || !slices.Equal(res.Config.SymlinkMap, want) {
		t.Fatalf("config %v, err %v", res.Config.SymlinkMap, res.Err)
	}
	saved, err := config.Load(path)
	if err != nil || !slices.Equal(saved.SymlinkMap, want) {
		t.Errorf("saved %v, %v", saved.SymlinkMap, err)
	}
	if _, err := os.Lstat(filepath.Join(cfg.LocalHomeDir, "Linked")); err != nil {
		t.Errorf("removing from the config touched the link: %v", err)
	}
}

func TestAddItem(t *testing.T) {
	cfg, path := fixture(t)
	must(t, os.WriteFile(filepath.Join(cfg.LocalHomeDir, ".bashrc"), nil, 0o644))

	got, item, err := AddItem(cfg, path, filepath.Join(cfg.LocalHomeDir, ".bashrc"))

	if err != nil || item != ".bashrc" || got.SymlinkMap[len(got.SymlinkMap)-1] != ".bashrc" {
		t.Fatalf("item %q, config %v, err %v", item, got.SymlinkMap, err)
	}
	if saved, _ := config.Load(path); !slices.Contains(saved.SymlinkMap, ".bashrc") {
		t.Errorf("not saved: %v", saved.SymlinkMap)
	}
	if _, _, err := AddItem(got, path, filepath.Join(cfg.LocalHomeDir, ".bashrc")); err == nil {
		t.Errorf("adding twice worked")
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
		got, err := NewItem(tt.path, home, cloud, existing)
		if tt.err == "" && (err != nil || got != tt.want) {
			t.Errorf("%s: got %q, %v; want %q", tt.path, got, err, tt.want)
		}
		if tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)) {
			t.Errorf("%s: err %v, want %q", tt.path, err, tt.err)
		}
	}
}
