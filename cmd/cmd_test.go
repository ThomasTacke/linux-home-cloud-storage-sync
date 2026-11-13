package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func init() {
	color.NoColor = true // plain text to compare against
}

// run executes the CLI with args and returns its output and error.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func writeConfig(t *testing.T, cloud, home string, items ...string) string {
	t.Helper()
	data, _ := json.Marshal(map[string]any{
		"cloudStorageHomeDir": cloud,
		"localHomeDir":        home,
		"symlinkMap":          items,
	})
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCreateSymlinksKeepsGoingAfterBrokenLink(t *testing.T) {
	root := t.TempDir()
	cloud, home := filepath.Join(root, "cloud"), filepath.Join(root, "home")
	for _, dir := range []string{filepath.Join(cloud, "Documents"), filepath.Join(cloud, "Music"), home} {
		os.MkdirAll(dir, 0o755)
	}
	os.Symlink(filepath.Join(root, "gone"), filepath.Join(home, "Documents"))
	config := writeConfig(t, cloud, home, "Documents", "Music")

	out, err := run(t, "create-symlinks", config)

	if err != nil {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if !strings.Contains(out, "broken symlink") {
		t.Errorf("no broken symlink notice:\n%s", out)
	}
	if dest, _ := os.Readlink(filepath.Join(home, "Music")); dest != filepath.Join(cloud, "Music") {
		t.Errorf("Music not linked after the broken entry:\n%s", out)
	}
}

func TestCreateSymlinksDryRunShortFlag(t *testing.T) {
	root := t.TempDir()
	cloud, home := filepath.Join(root, "cloud"), filepath.Join(root, "home")
	os.MkdirAll(filepath.Join(cloud, "Documents"), 0o755)
	config := writeConfig(t, cloud, home, "Documents")

	out, err := run(t, "create-symlinks", "-n", config)

	if err != nil || !strings.Contains(out, "would create symlink") {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if _, err := os.Lstat(filepath.Join(home, "Documents")); !os.IsNotExist(err) {
		t.Errorf("dry run created the link")
	}
}

func TestCreateSymlinksTildeInConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, "Nextcloud", "LinuxHome", "Documents"), 0o755)
	config := writeConfig(t, "~/Nextcloud/LinuxHome", "~", "Documents")

	if out, err := run(t, "create-symlinks", config); err != nil {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if dest, _ := os.Readlink(filepath.Join(home, "Documents")); dest != filepath.Join(home, "Nextcloud", "LinuxHome", "Documents") {
		t.Errorf("link points to %q", dest)
	}
}

func TestCreateSymlinksInvalidConfigFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte("{}"), 0o644)

	_, err := run(t, "create-symlinks", path)

	if err == nil || !strings.Contains(err.Error(), "symlinkMap: Field required") {
		t.Fatalf("err = %v, want the missing fields", err)
	}
}

func TestVersion(t *testing.T) {
	version = "v1.2.3"
	t.Cleanup(func() { version = "" })

	out, err := run(t, "--version")

	if err != nil || out != Name+" v1.2.3\n" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
}

func TestWriteDefaultConfigRefusesToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	if _, err := run(t, "write-default-config", path); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "write-default-config", path); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want already exists", err)
	}
}

func TestPrintTemplateConfigIsValidJSON(t *testing.T) {
	out, err := run(t, "print-template-config")
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if _, ok := parsed["symlinkMap"]; !ok {
		t.Errorf("no symlinkMap:\n%s", out)
	}
}
