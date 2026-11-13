package linker

import (
	"os"
	"path/filepath"
	"testing"
)

// setup returns a cloud folder with a "Documents" source and an empty home folder.
func setup(t *testing.T) (root, source, target string) {
	t.Helper()
	root = t.TempDir()
	source = filepath.Join(root, "cloud", "Documents")
	target = filepath.Join(root, "home", "Documents")
	must(t, os.MkdirAll(source, 0o755))
	must(t, os.MkdirAll(filepath.Dir(target), 0o755))
	return root, source, target
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func readlink(t *testing.T, path string) string {
	t.Helper()
	dest, err := os.Readlink(path)
	must(t, err)
	return dest
}

func TestCreatesMissingLink(t *testing.T) {
	_, source, target := setup(t)

	r := Link(source, target, Options{})

	if r.State != Created {
		t.Fatalf("state = %v, want Created (err %v)", r.State, r.Err)
	}
	if got := readlink(t, target); got != source {
		t.Errorf("link points to %q, want %q", got, source)
	}
}

func TestCreatesParentFolders(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cloud", ".config", "fish")
	target := filepath.Join(root, "home", ".config", "fish")
	must(t, os.MkdirAll(source, 0o755))

	if r := Link(source, target, Options{}); r.State != Created {
		t.Fatalf("state = %v, want Created (err %v)", r.State, r.Err)
	}
}

func TestDryRunCreatesNothing(t *testing.T) {
	_, source, target := setup(t)

	if r := Link(source, target, Options{DryRun: true}); r.State != WouldCreate {
		t.Fatalf("state = %v, want WouldCreate", r.State)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("target exists after dry run: %v", err)
	}
}

func TestSourceMissing(t *testing.T) {
	root := t.TempDir()

	r := Link(filepath.Join(root, "cloud", "Nope"), filepath.Join(root, "home", "Nope"), Options{})

	if r.State != SourceMissing {
		t.Fatalf("state = %v, want SourceMissing", r.State)
	}
}

func TestAlreadyLinked(t *testing.T) {
	_, source, target := setup(t)
	must(t, os.Symlink(source, target))

	if r := Link(source, target, Options{}); r.State != AlreadyLinked {
		t.Fatalf("state = %v, want AlreadyLinked", r.State)
	}
}

func TestRelativeLinkToSourceCountsAsLinked(t *testing.T) {
	_, source, target := setup(t)
	must(t, os.Symlink(filepath.Join("..", "cloud", "Documents"), target))

	if r := Link(source, target, Options{}); r.State != AlreadyLinked {
		t.Fatalf("state = %v, want AlreadyLinked", r.State)
	}
}

// A broken link must be reported and left alone, not taken for "missing".
func TestBrokenLinkIsLeftAlone(t *testing.T) {
	root, source, target := setup(t)
	gone := filepath.Join(root, "gone")
	must(t, os.Symlink(gone, target))

	r := Link(source, target, Options{})

	if r.State != BrokenLink {
		t.Fatalf("state = %v, want BrokenLink (err %v)", r.State, r.Err)
	}
	if r.PointsTo != gone {
		t.Errorf("PointsTo = %q, want %q", r.PointsTo, gone)
	}
	if got := readlink(t, target); got != gone {
		t.Errorf("link was changed to %q", got)
	}
}

// A link to somewhere else must not be reported as fine.
func TestLinkElsewhereIsLeftAlone(t *testing.T) {
	root, source, target := setup(t)
	elsewhere := filepath.Join(root, "elsewhere")
	must(t, os.Mkdir(elsewhere, 0o755))
	must(t, os.Symlink(elsewhere, target))

	r := Link(source, target, Options{})

	if r.State != LinkedElsewhere {
		t.Fatalf("state = %v, want LinkedElsewhere", r.State)
	}
	if r.PointsTo != elsewhere {
		t.Errorf("PointsTo = %q, want %q", r.PointsTo, elsewhere)
	}
}

func TestExistingDirectoryAndFileAreLeftAlone(t *testing.T) {
	_, source, target := setup(t)
	must(t, os.Mkdir(target, 0o755))

	if r := Link(source, target, Options{}); r.State != ExistsDir {
		t.Fatalf("state = %v, want ExistsDir", r.State)
	}

	file := filepath.Join(filepath.Dir(target), "file")
	must(t, os.WriteFile(file, nil, 0o644))
	if r := Link(source, file, Options{}); r.State != ExistsFile {
		t.Fatalf("state = %v, want ExistsFile", r.State)
	}
}

func TestFailureIsReportedNotPanicking(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	_, source, target := setup(t)
	home := filepath.Dir(target)
	must(t, os.Chmod(home, 0o555))
	t.Cleanup(func() { os.Chmod(home, 0o755) })

	r := Link(source, target, Options{})

	if r.State != Failed || r.Err == nil {
		t.Fatalf("state = %v, err = %v, want Failed with an error", r.State, r.Err)
	}
}
