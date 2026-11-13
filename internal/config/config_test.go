package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseValid(t *testing.T) {
	cfg, err := Parse([]byte(`{"cloudStorageHomeDir": "/c", "localHomeDir": "/h", "symlinkMap": ["Documents", ".config/fish"]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{CloudStorageHomeDir: "/c", LocalHomeDir: "/h", SymlinkMap: []string{"Documents", ".config/fish"}}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestParseExpandsTilde(t *testing.T) {
	t.Setenv("HOME", "/home/someone")

	cfg, err := Parse([]byte(`{"cloudStorageHomeDir": "~/Nextcloud/LinuxHome", "localHomeDir": "~", "symlinkMap": []}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CloudStorageHomeDir != "/home/someone/Nextcloud/LinuxHome" || cfg.LocalHomeDir != "/home/someone" {
		t.Errorf("got %+v", cfg)
	}
}

func TestParseReportsEveryProblem(t *testing.T) {
	tests := []struct {
		name string
		json string
		want []string
	}{
		{"empty object", `{}`, []string{
			"cloudStorageHomeDir: Field required",
			"localHomeDir: Field required",
			"symlinkMap: Field required",
		}},
		{"wrong types", `{"cloudStorageHomeDir": 1, "localHomeDir": "/h", "symlinkMap": "Documents"}`, []string{
			"cloudStorageHomeDir: Input should be a valid string",
			"symlinkMap: Input should be a list of strings",
		}},
		{"empty path", `{"cloudStorageHomeDir": "", "localHomeDir": "/h", "symlinkMap": []}`, []string{
			"cloudStorageHomeDir: must not be empty",
		}},
		{"not json", `nope`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.json))
			var invalid *ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("err = %v, want a ValidationError", err)
			}
			if tt.want != nil && !reflect.DeepEqual(invalid.Problems, tt.want) {
				t.Errorf("problems = %q, want %q", invalid.Problems, tt.want)
			}
		})
	}
}

func TestWriteAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	cfg := Config{CloudStorageHomeDir: "/c", LocalHomeDir: "/h", SymlinkMap: []string{"Music"}}

	if err := cfg.Write(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, cfg) {
		t.Errorf("loaded %+v, want %+v", loaded, cfg)
	}
}

func TestWriteNeverOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Defaults().Write(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("err = %v, want ErrExist", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "mine" {
		t.Errorf("file was overwritten: %q", data)
	}
}

func TestSaveSymlinkMapKeepsTheRestAndTheLink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "cloud", "config.json")
	link := filepath.Join(dir, "home", ".config", "config.json")
	os.MkdirAll(filepath.Dir(real), 0o755)
	os.MkdirAll(filepath.Dir(link), 0o755)
	original := `{"cloudStorageHomeDir": "~/Nextcloud/LinuxHome", "localHomeDir": "~", "symlinkMap": ["Documents"], "comment": "mine"}`
	os.WriteFile(real, []byte(original), 0o600)
	os.Symlink(real, link)

	cfg, err := Load(link)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveSymlinkMap(link, cfg, []string{"Documents", ".gitconfig"}); err != nil {
		t.Fatal(err)
	}

	if dest, err := os.Readlink(link); err != nil || dest != real {
		t.Fatalf("the link was replaced: %q, %v", dest, err)
	}
	var raw map[string]any
	data, _ := os.ReadFile(real)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, data)
	}
	if raw["cloudStorageHomeDir"] != "~/Nextcloud/LinuxHome" || raw["localHomeDir"] != "~" || raw["comment"] != "mine" {
		t.Errorf("other keys changed:\n%s", data)
	}
	if got := raw["symlinkMap"]; !reflect.DeepEqual(got, []any{"Documents", ".gitconfig"}) {
		t.Errorf("symlinkMap = %v", got)
	}
	if info, _ := os.Stat(real); info.Mode().Perm() != 0o600 {
		t.Errorf("permissions changed to %v", info.Mode().Perm())
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, "cloud", ".config.json.*")); len(leftovers) != 0 {
		t.Errorf("temporary files left: %v", leftovers)
	}
}

func TestSaveSymlinkMapCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	cfg := Config{CloudStorageHomeDir: "/c", LocalHomeDir: "/h"}

	if err := SaveSymlinkMap(path, cfg, []string{"Music"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || !reflect.DeepEqual(loaded.SymlinkMap, []string{"Music"}) {
		t.Fatalf("loaded %+v, %v", loaded, err)
	}
}
