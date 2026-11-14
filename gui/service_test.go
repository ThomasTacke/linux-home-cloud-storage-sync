package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// fixture: a cloud and a home folder with one entry per state, and a SyncService on a
// config file for them.
func fixture(t *testing.T) (*SyncService, config.Config) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	cloud, home := filepath.Join(root, "Nextcloud"), filepath.Join(root, "home")
	mkdir := func(p string) { must(t, os.MkdirAll(p, 0o755)) }
	mkdir(filepath.Join(cloud, "Documents")) // synced
	mkdir(home)
	must(t, os.Symlink(filepath.Join(cloud, "Documents"), filepath.Join(home, "Documents")))
	mkdir(filepath.Join(cloud, "Pictures"))                                           // ready
	must(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]"), 0o644)) // local only
	mkdir(filepath.Join(cloud, "Music"))                                              // conflict
	mkdir(filepath.Join(home, "Music"))
	mkdir(filepath.Join(cloud, "Videos")) // broken
	must(t, os.Symlink(filepath.Join(root, "gone"), filepath.Join(home, "Videos")))
	cfg := config.Config{
		CloudStorageHomeDir: cloud,
		LocalHomeDir:        home,
		SymlinkMap:          []string{"Documents", "Pictures", ".gitconfig", "Music", "Videos", "Templates"},
	}
	path := filepath.Join(root, "config.json")
	must(t, cfg.Write(path))
	return &SyncService{configPath: path}, cfg
}

func TestOverviewStatuses(t *testing.T) {
	s, _ := fixture(t)
	o, err := s.Overview()
	must(t, err)
	want := map[string]Status{
		"Documents": StatusSynced, "Pictures": StatusReady, ".gitconfig": StatusLocalOnly,
		"Music": StatusConflict, "Videos": StatusBroken, "Templates": StatusMissing,
	}
	for _, e := range o.Entries {
		if e.Status != want[e.Item] {
			t.Errorf("%s: %s, want %s", e.Item, e.Status, want[e.Item])
		}
	}
	if o.CloudDir != "~/Nextcloud" || !o.Entries[2].Hidden || o.Entries[0].Hidden {
		t.Errorf("cloudDir %q, hidden flags %v %v", o.CloudDir, o.Entries[2].Hidden, o.Entries[0].Hidden)
	}
}

func TestPreviewFixChangesNothing(t *testing.T) {
	s, _ := fixture(t)
	before, _ := s.Overview()

	p, err := s.Preview(ActionFix, []string{"Documents", "Pictures", ".gitconfig", "Music", "Videos", "Templates"})
	must(t, err)

	if p.Changes != 4 {
		t.Fatalf("changes %d, want 4", p.Changes)
	}
	want := []Outcome{OutcomeSkipSynced, OutcomeConnect, OutcomeMove, OutcomeBackup, OutcomeRepair, OutcomeSkipNoCloud}
	for i, st := range p.Steps {
		if st.Outcome != want[i] {
			t.Errorf("%s: %s, want %s", st.Item, st.Outcome, want[i])
		}
	}
	if p.Steps[3].Backup == "" || p.Steps[4].PointsTo != "~/gone" {
		t.Errorf("backup %q, pointsTo %q", p.Steps[3].Backup, p.Steps[4].PointsTo)
	}
	after, _ := s.Overview()
	for i := range before.Entries {
		if before.Entries[i].Status != after.Entries[i].Status {
			t.Errorf("preview changed %s", before.Entries[i].Item)
		}
	}
}

func TestApplyFix(t *testing.T) {
	s, cfg := fixture(t)
	res, err := s.Apply(ActionFix, []string{"Pictures", ".gitconfig", "Music", "Videos"})
	must(t, err)
	if len(res.Steps) != 4 || res.Failed != 0 {
		t.Fatalf("steps %d, failed %d: %+v", len(res.Steps), res.Failed, res.Steps)
	}
	o, _ := s.Overview()
	for _, e := range o.Entries {
		if e.Item != "Templates" && e.Status != StatusSynced {
			t.Errorf("%s: %s after fix", e.Item, e.Status)
		}
	}
	if backups, _ := filepath.Glob(filepath.Join(cfg.LocalHomeDir, "Music.backup-*")); len(backups) != 1 {
		t.Errorf("backups %v", backups)
	}
}

func TestPreviewOnlyActsOnTheChosenAction(t *testing.T) {
	s, _ := fixture(t)
	// Connect only links; the local copy and the wrong link are skipped
	p, err := s.Preview(ActionConnect, []string{"Pictures", "Music", "Videos"})
	must(t, err)
	got := []Outcome{p.Steps[0].Outcome, p.Steps[1].Outcome, p.Steps[2].Outcome}
	want := []Outcome{OutcomeConnect, OutcomeSkipLocal, OutcomeSkipLink}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d: %s, want %s", i, got[i], want[i])
		}
	}
}

func TestStopSyncingKeepsACopy(t *testing.T) {
	s, cfg := fixture(t)
	res, err := s.Apply(ActionStop, []string{"Documents"})
	must(t, err)
	if len(res.Steps) != 1 || res.Steps[0].Outcome != OutcomeCopyBack || !res.Steps[0].Change {
		t.Fatalf("result %+v", res.Steps)
	}
	if info, err := os.Lstat(filepath.Join(cfg.LocalHomeDir, "Documents")); err != nil || !info.IsDir() {
		t.Errorf("not a real folder now: %v", err)
	}
}

func TestRemoveAndAddEntry(t *testing.T) {
	s, cfg := fixture(t)
	res, err := s.Apply(ActionRemove, []string{"Templates"})
	must(t, err)
	if res.Error != "" || len(res.Steps) != 1 {
		t.Fatalf("remove: %+v", res)
	}
	must(t, os.MkdirAll(filepath.Join(cfg.LocalHomeDir, "Projects"), 0o755))

	added, err := s.AddEntry(filepath.Join(cfg.LocalHomeDir, "Projects"))
	must(t, err)
	if added.Item != "Projects" || added.Entry.Status != StatusLocalOnly || !added.Entry.IsDir {
		t.Errorf("added %+v", added)
	}
	if _, err := s.AddEntry(filepath.Join(cfg.LocalHomeDir, "Projects")); err == nil {
		t.Errorf("adding twice worked")
	}
	o, _ := s.Overview()
	if len(o.Entries) != 6 || o.Entries[5].Item != "Projects" {
		t.Errorf("entries %v", o.Entries)
	}
}

func TestUnknownAction(t *testing.T) {
	s, _ := fixture(t)
	if _, err := s.Preview("explode", []string{"Documents"}); err == nil {
		t.Error("unknown action accepted")
	}
}

func TestNeedsSetup(t *testing.T) {
	s := &SyncService{configPath: filepath.Join(t.TempDir(), "missing.json")}
	o, err := s.Overview()
	if err != nil || !o.NeedsSetup {
		t.Errorf("needsSetup %v, err %v", o.NeedsSetup, err)
	}
}
