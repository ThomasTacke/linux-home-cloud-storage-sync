package linker

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	must(t, os.WriteFile(path, []byte(content), perm))
	must(t, os.Chmod(path, perm))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	return string(data)
}

func assertLinksTo(t *testing.T, target, source string) {
	t.Helper()
	if got := readlink(t, target); got != source {
		t.Fatalf("%s points to %q, want %q", target, got, source)
	}
}

var fixedNow = func() time.Time { return time.Date(2026, 11, 13, 21, 5, 9, 0, time.UTC) }

// ---- --adopt ----

func TestAdoptMovesFolderIntoCloudAndLinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cloud", ".config", "fish")
	target := filepath.Join(root, "home", ".config", "fish")
	writeFile(t, filepath.Join(target, "config.fish"), "set -g x 1", 0o600)

	r := Link(source, target, Options{Adopt: true})

	if r.State != Adopted {
		t.Fatalf("state = %v, want Adopted (err %v)", r.State, r.Err)
	}
	assertLinksTo(t, target, source)
	if got := readFile(t, filepath.Join(source, "config.fish")); got != "set -g x 1" {
		t.Errorf("content in cloud = %q", got)
	}
	if info, _ := os.Stat(filepath.Join(source, "config.fish")); info.Mode().Perm() != 0o600 {
		t.Errorf("permissions changed to %v", info.Mode().Perm())
	}
}

func TestAdoptMovesFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cloud", ".gitconfig")
	target := filepath.Join(root, "home", ".gitconfig")
	writeFile(t, target, "[user]", 0o644)

	if r := Link(source, target, Options{Adopt: true}); r.State != Adopted {
		t.Fatalf("state = %v, want Adopted (err %v)", r.State, r.Err)
	}
	assertLinksTo(t, target, source)
	if got := readFile(t, target); got != "[user]" {
		t.Errorf("content through link = %q", got)
	}
}

func TestAdoptDryRunChangesNothing(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cloud", ".gitconfig")
	target := filepath.Join(root, "home", ".gitconfig")
	writeFile(t, target, "[user]", 0o644)

	if r := Link(source, target, Options{Adopt: true, DryRun: true}); r.State != WouldAdopt {
		t.Fatalf("state = %v, want WouldAdopt", r.State)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Errorf("source created in dry run")
	}
	if got := readFile(t, target); got != "[user]" {
		t.Errorf("target changed in dry run: %q", got)
	}
}

func TestWithoutAdoptLocalCopyStays(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cloud", ".gitconfig")
	target := filepath.Join(root, "home", ".gitconfig")
	writeFile(t, target, "[user]", 0o644)

	if r := Link(source, target, Options{}); r.State != SourceMissing {
		t.Fatalf("state = %v, want SourceMissing", r.State)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Errorf("moved without --adopt")
	}
}

// A symlink is not adopted: moving it into the cloud would sync a link, not the data.
func TestAdoptLeavesSymlinkAlone(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cloud", "Music")
	target := filepath.Join(root, "home", "Music")
	must(t, os.MkdirAll(filepath.Join(root, "elsewhere"), 0o755))
	must(t, os.MkdirAll(filepath.Dir(target), 0o755))
	must(t, os.Symlink(filepath.Join(root, "elsewhere"), target))

	if r := Link(source, target, Options{Adopt: true, Force: true}); r.State != SourceMissing {
		t.Fatalf("state = %v, want SourceMissing", r.State)
	}
	assertLinksTo(t, target, filepath.Join(root, "elsewhere"))
}

// When the cloud already has the entry, --adopt alone doesn't touch the local copy.
func TestAdoptDoesNotOverwriteCloudCopy(t *testing.T) {
	_, source, target := setup(t)
	writeFile(t, filepath.Join(target, "local.txt"), "local", 0o644)

	if r := Link(source, target, Options{Adopt: true}); r.State != ExistsDir {
		t.Fatalf("state = %v, want ExistsDir", r.State)
	}
	if got := readFile(t, filepath.Join(target, "local.txt")); got != "local" {
		t.Errorf("local copy changed")
	}
}

// ---- --force ----

func TestForceBacksUpLocalCopyAndLinks(t *testing.T) {
	_, source, target := setup(t)
	writeFile(t, filepath.Join(target, "local.txt"), "local", 0o644)

	r := Link(source, target, Options{Force: true, Now: fixedNow})

	if r.State != BackedUp {
		t.Fatalf("state = %v, want BackedUp (err %v)", r.State, r.Err)
	}
	wantBackup := target + ".backup-20261113-210509"
	if r.Backup != wantBackup {
		t.Errorf("Backup = %q, want %q", r.Backup, wantBackup)
	}
	assertLinksTo(t, target, source)
	if got := readFile(t, filepath.Join(wantBackup, "local.txt")); got != "local" {
		t.Errorf("backup content = %q", got)
	}
}

func TestForceDryRunBacksUpNothing(t *testing.T) {
	_, source, target := setup(t)
	writeFile(t, filepath.Join(target, "local.txt"), "local", 0o644)

	r := Link(source, target, Options{Force: true, DryRun: true, Now: fixedNow})

	if r.State != WouldBackUp {
		t.Fatalf("state = %v, want WouldBackUp", r.State)
	}
	if _, err := os.Lstat(r.Backup); !os.IsNotExist(err) {
		t.Errorf("backup created in dry run")
	}
	if got := readFile(t, filepath.Join(target, "local.txt")); got != "local" {
		t.Errorf("local copy changed in dry run")
	}
}

func TestForceRefusesToOverwriteExistingBackup(t *testing.T) {
	_, source, target := setup(t)
	writeFile(t, target, "local", 0o644)
	writeFile(t, target+".backup-20261113-210509", "older backup", 0o644)

	r := Link(source, target, Options{Force: true, Now: fixedNow})

	if r.State != Failed {
		t.Fatalf("state = %v, want Failed", r.State)
	}
	if got := readFile(t, target+".backup-20261113-210509"); got != "older backup" {
		t.Errorf("existing backup overwritten")
	}
	if got := readFile(t, target); got != "local" {
		t.Errorf("local copy changed")
	}
}

func TestForceReplacesBrokenLink(t *testing.T) {
	root, source, target := setup(t)
	must(t, os.Symlink(filepath.Join(root, "gone"), target))

	r := Link(source, target, Options{Force: true})

	if r.State != Replaced {
		t.Fatalf("state = %v, want Replaced (err %v)", r.State, r.Err)
	}
	assertLinksTo(t, target, source)
	if _, err := os.Lstat(target + ".lhcss-tmp"); !os.IsNotExist(err) {
		t.Errorf("temporary link left behind")
	}
}

func TestForceReplacesLinkElsewhereButNotItsDestination(t *testing.T) {
	root, source, target := setup(t)
	elsewhere := filepath.Join(root, "elsewhere")
	writeFile(t, filepath.Join(elsewhere, "keep.txt"), "keep", 0o644)
	must(t, os.Symlink(elsewhere, target))

	if r := Link(source, target, Options{Force: true}); r.State != Replaced {
		t.Fatalf("state = %v, want Replaced", r.State)
	}
	assertLinksTo(t, target, source)
	if got := readFile(t, filepath.Join(elsewhere, "keep.txt")); got != "keep" {
		t.Errorf("destination of the old link changed")
	}
}

func TestForceDryRunKeepsLink(t *testing.T) {
	root, source, target := setup(t)
	must(t, os.Symlink(filepath.Join(root, "gone"), target))

	if r := Link(source, target, Options{Force: true, DryRun: true}); r.State != WouldReplace {
		t.Fatalf("state = %v, want WouldReplace", r.State)
	}
	assertLinksTo(t, target, filepath.Join(root, "gone"))
}

func TestForceKeepsCorrectLink(t *testing.T) {
	_, source, target := setup(t)
	must(t, os.Symlink(source, target))

	if r := Link(source, target, Options{Force: true}); r.State != AlreadyLinked {
		t.Fatalf("state = %v, want AlreadyLinked", r.State)
	}
}

func TestForceLeavesSocketAlone(t *testing.T) {
	_, source, _ := setup(t)
	// Unix socket paths are limited to ~108 bytes, so use a short temp dir
	dir, err := os.MkdirTemp("", "lhcss")
	must(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	target := filepath.Join(dir, "s")
	l, err := net.Listen("unix", target)
	must(t, err)
	t.Cleanup(func() { l.Close() })

	if r := Link(source, target, Options{Force: true}); r.State != ExistsOther {
		t.Fatalf("state = %v, want ExistsOther", r.State)
	}
}

// ---- Unlink ----

func TestUnlinkRemovesOurLink(t *testing.T) {
	_, source, target := setup(t)
	must(t, os.Symlink(source, target))

	if r := Unlink(source, target, UnlinkOptions{}); r.State != Removed {
		t.Fatalf("state = %v, want Removed (err %v)", r.State, r.Err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("link still there")
	}
	if _, err := os.Stat(source); err != nil {
		t.Errorf("cloud copy gone: %v", err)
	}
}

// A link to the entry whose cloud copy is gone is still ours and removed.
func TestUnlinkRemovesOurBrokenLink(t *testing.T) {
	_, source, target := setup(t)
	must(t, os.Symlink(source, target))
	must(t, os.RemoveAll(source))

	if r := Unlink(source, target, UnlinkOptions{}); r.State != Removed {
		t.Fatalf("state = %v, want Removed (err %v)", r.State, r.Err)
	}
}

func TestUnlinkDryRunKeepsLink(t *testing.T) {
	_, source, target := setup(t)
	must(t, os.Symlink(source, target))

	if r := Unlink(source, target, UnlinkOptions{DryRun: true}); r.State != WouldRemove {
		t.Fatalf("state = %v, want WouldRemove", r.State)
	}
	assertLinksTo(t, target, source)
}

func TestUnlinkLeavesOtherThingsAlone(t *testing.T) {
	root, source, target := setup(t)

	// Nothing there
	if r := Unlink(source, target, UnlinkOptions{}); r.State != NothingThere {
		t.Fatalf("state = %v, want NothingThere", r.State)
	}
	// A link to elsewhere
	must(t, os.Symlink(filepath.Join(root, "elsewhere"), target))
	if r := Unlink(source, target, UnlinkOptions{Restore: true}); r.State != LinkedElsewhere {
		t.Fatalf("state = %v, want LinkedElsewhere", r.State)
	}
	assertLinksTo(t, target, filepath.Join(root, "elsewhere"))
	// A real folder
	must(t, os.Remove(target))
	writeFile(t, filepath.Join(target, "mine.txt"), "mine", 0o644)
	if r := Unlink(source, target, UnlinkOptions{Restore: true}); r.State != ExistsDir {
		t.Fatalf("state = %v, want ExistsDir", r.State)
	}
	if got := readFile(t, filepath.Join(target, "mine.txt")); got != "mine" {
		t.Errorf("real folder changed")
	}
}

func TestUnlinkRestoreCopiesFolderWithPermissionsAndLinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cloud", ".ssh")
	target := filepath.Join(root, "home", ".ssh")
	writeFile(t, filepath.Join(source, "id_ed25519"), "private", 0o600)
	writeFile(t, filepath.Join(source, "id_ed25519.pub"), "public", 0o644)
	must(t, os.Symlink("id_ed25519.pub", filepath.Join(source, "current.pub")))
	must(t, os.Chmod(source, 0o700))
	must(t, os.MkdirAll(filepath.Dir(target), 0o755))
	must(t, os.Symlink(source, target))

	r := Unlink(source, target, UnlinkOptions{Restore: true})

	if r.State != Restored {
		t.Fatalf("state = %v, want Restored (err %v)", r.State, r.Err)
	}
	info, err := os.Lstat(target)
	must(t, err)
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Errorf("target is %v, want a 0700 directory", info.Mode())
	}
	if info, _ := os.Stat(filepath.Join(target, "id_ed25519")); info.Mode().Perm() != 0o600 {
		t.Errorf("key permissions = %v, want 0600", info.Mode().Perm())
	}
	if got := readlink(t, filepath.Join(target, "current.pub")); got != "id_ed25519.pub" {
		t.Errorf("inner symlink = %q", got)
	}
	if got := readFile(t, filepath.Join(source, "id_ed25519")); got != "private" {
		t.Errorf("cloud copy changed")
	}
	if _, err := os.Lstat(target + ".lhcss-restore"); !os.IsNotExist(err) {
		t.Errorf("temporary copy left behind")
	}
}

func TestUnlinkRestoreCopiesFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cloud", ".gitconfig")
	target := filepath.Join(root, "home", ".gitconfig")
	writeFile(t, source, "[user]", 0o644)
	must(t, os.MkdirAll(filepath.Dir(target), 0o755))
	must(t, os.Symlink(source, target))

	if r := Unlink(source, target, UnlinkOptions{Restore: true}); r.State != Restored {
		t.Fatalf("state = %v, want Restored (err %v)", r.State, r.Err)
	}
	if info, _ := os.Lstat(target); !info.Mode().IsRegular() {
		t.Fatalf("target is %v, want a regular file", info.Mode())
	}
	if got := readFile(t, target); got != "[user]" {
		t.Errorf("content = %q", got)
	}
}

func TestUnlinkRestoreWithoutCloudCopyKeepsLink(t *testing.T) {
	_, source, target := setup(t)
	must(t, os.Symlink(source, target))
	must(t, os.RemoveAll(source))

	if r := Unlink(source, target, UnlinkOptions{Restore: true}); r.State != SourceMissing {
		t.Fatalf("state = %v, want SourceMissing", r.State)
	}
	assertLinksTo(t, target, source)
}
