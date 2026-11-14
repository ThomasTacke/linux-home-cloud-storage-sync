package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
)

// freshHome: a home with German folder names and a Nextcloud folder, no config yet.
func freshHome(t *testing.T) (*SyncService, string, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	home := root
	must(t, os.MkdirAll(filepath.Join(home, ".config"), 0o755))
	must(t, os.WriteFile(filepath.Join(home, ".config", "user-dirs.dirs"), []byte(`# written by xdg-user-dirs-update
XDG_DESKTOP_DIR="$HOME/Schreibtisch"
XDG_DOWNLOAD_DIR="$HOME/Downloads"
XDG_DOCUMENTS_DIR="$HOME/Dokumente"
XDG_PICTURES_DIR="$HOME/Bilder"
XDG_MUSIC_DIR="$HOME/Musik"
XDG_VIDEOS_DIR="$HOME/Videos"
XDG_TEMPLATES_DIR="$HOME/"
`), 0o644))
	for _, d := range []string{"Schreibtisch", "Dokumente", "Bilder", "Musik"} {
		must(t, os.MkdirAll(filepath.Join(home, d), 0o755))
	}
	must(t, os.WriteFile(filepath.Join(home, "Dokumente", "brief.txt"), []byte("hallo"), 0o644))
	cloud := filepath.Join(home, "Nextcloud")
	must(t, os.MkdirAll(cloud, 0o755))
	return &SyncService{configPath: filepath.Join(home, ".config", "linux-home-cloud-storage-sync", "config.json"), homeDir: home}, home, cloud
}

func TestSetupInfoUsesTheSystemsFolderNames(t *testing.T) {
	s, _, cloud := freshHome(t)
	info, err := s.SetupInfo()
	must(t, err)

	var items []string
	for _, f := range info.Folders {
		items = append(items, f.Item)
	}
	// Templates is "$HOME/" (unset) and left out, Downloads is never offered
	want := []string{"Schreibtisch", "Dokumente", "Bilder", "Musik", "Videos"}
	if !slices.Equal(items, want) {
		t.Errorf("folders %v, want %v", items, want)
	}
	if !info.Folders[1].Exists || info.Folders[4].Exists {
		t.Errorf("exists flags: %+v", info.Folders)
	}
	if len(info.Clouds) != 1 || info.Clouds[0].Path != cloud || info.Clouds[0].Show != "~/Nextcloud" {
		t.Errorf("clouds %+v", info.Clouds)
	}
}

func TestEnglishFallbackAndNextcloudClientConfig(t *testing.T) {
	root := t.TempDir()
	elsewhere := filepath.Join(root, "data", "MyCloud")
	must(t, os.MkdirAll(elsewhere, 0o755))
	must(t, os.MkdirAll(filepath.Join(root, ".config", "Nextcloud"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, ".config", "Nextcloud", "nextcloud.cfg"),
		[]byte("[Accounts]\n0\\Folders\\1\\localPath="+elsewhere+"/\n0\\Folders\\1\\paused=false\n"), 0o644))

	folders := userFolders(root)
	if folders[1].Item != "Documents" || len(folders) != 6 {
		t.Errorf("fallback folders %+v", folders)
	}
	clouds := findClouds(root)
	if len(clouds) != 1 || clouds[0].Path != elsewhere {
		t.Errorf("clouds %+v", clouds)
	}
}

func TestSetupFirstComputerMovesFoldersIntoTheCloud(t *testing.T) {
	s, home, cloud := freshHome(t)
	items := []string{"Dokumente", "Bilder"}

	p, err := s.SetupPreview(cloud, items)
	must(t, err)
	if p.Changes != 2 || p.Steps[0].Outcome != OutcomeMove {
		t.Fatalf("preview %+v", p)
	}
	if _, err := os.Stat(s.path()); !os.IsNotExist(err) {
		t.Fatalf("preview wrote the config")
	}

	res, err := s.SetupApply(cloud, items)
	must(t, err)
	if res.Failed != 0 || len(res.Steps) != 2 {
		t.Fatalf("result %+v", res)
	}
	if dest, _ := os.Readlink(filepath.Join(home, "Dokumente")); dest != filepath.Join(cloud, "LinuxHome", "Dokumente") {
		t.Errorf("Dokumente points to %q", dest)
	}
	if data, _ := os.ReadFile(filepath.Join(cloud, "LinuxHome", "Dokumente", "brief.txt")); string(data) != "hallo" {
		t.Errorf("content moved wrong: %q", data)
	}
	// The file uses ~ so the same config works on every computer
	raw, _ := os.ReadFile(s.path())
	if !strings.Contains(string(raw), `"~/Nextcloud/LinuxHome"`) || !strings.Contains(string(raw), `"localHomeDir": "~"`) {
		t.Errorf("config:\n%s", raw)
	}
	o, err := s.Overview()
	must(t, err)
	if o.NeedsSetup || len(o.Entries) != 2 || o.Entries[0].Status != StatusSynced {
		t.Errorf("overview after setup %+v", o)
	}
	if _, err := s.SetupApply(cloud, items); err == nil {
		t.Errorf("set up twice")
	}
}

func TestSetupSecondComputerKeepsItsCopyAsBackup(t *testing.T) {
	s, home, cloud := freshHome(t)
	// The first computer already put Dokumente into the cloud
	must(t, os.MkdirAll(filepath.Join(cloud, "LinuxHome", "Dokumente"), 0o755))

	p, err := s.SetupPreview(cloud, []string{"Dokumente"})
	must(t, err)
	if p.Steps[0].Outcome != OutcomeBackup || !strings.HasPrefix(p.Steps[0].Backup, "Dokumente.backup-") {
		t.Fatalf("preview %+v", p.Steps)
	}
	_, err = s.SetupApply(cloud, []string{"Dokumente"})
	must(t, err)
	backups, _ := filepath.Glob(filepath.Join(home, "Dokumente.backup-*"))
	if len(backups) != 1 {
		t.Fatalf("backups %v", backups)
	}
	if data, _ := os.ReadFile(filepath.Join(backups[0], "brief.txt")); string(data) != "hallo" {
		t.Errorf("the local copy is gone")
	}
}

func TestSetupRejectsAMissingCloudFolder(t *testing.T) {
	s, home, _ := freshHome(t)
	if _, err := s.SetupPreview(filepath.Join(home, "nope"), []string{"Dokumente"}); err == nil {
		t.Error("accepted a missing cloud folder")
	}
	if _, err := s.SetupPreview("relative/path", nil); err == nil {
		t.Error("accepted a relative path")
	}
	if _, err := config.Load(s.path()); !os.IsNotExist(err) {
		t.Errorf("config written: %v", err)
	}
}
