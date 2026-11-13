// Package config reads and writes the JSON configuration: where the cloud copy lives,
// where the home is, and which entries to link.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the JSON file, with the same camelCase keys as the Python version.
type Config struct {
	CloudStorageHomeDir string   `json:"cloudStorageHomeDir"`
	LocalHomeDir        string   `json:"localHomeDir"`
	SymlinkMap          []string `json:"symlinkMap"`
}

// ValidationError lists every problem with a config file, one "field: message" each.
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return strings.Join(e.Problems, "\n")
}

func home() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "~"
	}
	return dir
}

// DefaultPath is where the config is looked for when no path is given.
func DefaultPath() string {
	return filepath.Join(home(), ".config", "linux-home-cloud-storage-sync", "config.json")
}

// Defaults is the template configuration.
func Defaults() Config {
	return Config{
		CloudStorageHomeDir: filepath.Join(home(), "Nextcloud", "LinuxHome"),
		LocalHomeDir:        home(),
		SymlinkMap:          []string{"Desktop", "Documents", "Music", "Pictures", "Templates", "Videos"},
	}
}

// ExpandHome turns a leading "~" into the home of the user running the tool.
func ExpandHome(path string) string {
	if path == "~" {
		return home()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home(), path[2:])
	}
	return path
}

// Load reads and validates a config file and expands "~" in its paths.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(data)
}

// Parse validates JSON config data and expands "~" in its paths.
func Parse(data []byte) (Config, error) {
	// Decode into raw values first, so missing fields and wrong types can be reported per
	// field instead of as one decoder error
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, &ValidationError{Problems: []string{"(file): " + err.Error()}}
	}

	var cfg Config
	var problems []string
	field := func(name string, target any) {
		value, ok := raw[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			problems = append(problems, name+": Field required")
			return
		}
		if err := json.Unmarshal(value, target); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %s", name, typeMessage(target)))
		}
	}
	field("cloudStorageHomeDir", &cfg.CloudStorageHomeDir)
	field("localHomeDir", &cfg.LocalHomeDir)
	field("symlinkMap", &cfg.SymlinkMap)

	if _, ok := raw["cloudStorageHomeDir"]; ok && cfg.CloudStorageHomeDir == "" && !contains(problems, "cloudStorageHomeDir") {
		problems = append(problems, "cloudStorageHomeDir: must not be empty")
	}
	if _, ok := raw["localHomeDir"]; ok && cfg.LocalHomeDir == "" && !contains(problems, "localHomeDir") {
		problems = append(problems, "localHomeDir: must not be empty")
	}
	if len(problems) > 0 {
		return Config{}, &ValidationError{Problems: problems}
	}

	cfg.CloudStorageHomeDir = ExpandHome(cfg.CloudStorageHomeDir)
	cfg.LocalHomeDir = ExpandHome(cfg.LocalHomeDir)
	return cfg, nil
}

func typeMessage(target any) string {
	if _, ok := target.(*[]string); ok {
		return "Input should be a list of strings"
	}
	return "Input should be a valid string"
}

func contains(problems []string, field string) bool {
	for _, p := range problems {
		if strings.HasPrefix(p, field+":") {
			return true
		}
	}
	return false
}

// JSON returns the config as indented JSON.
func (c Config) JSON() string {
	data, _ := json.MarshalIndent(c, "", "    ")
	return string(data)
}

// Write saves the config, creating the folder. It refuses to overwrite an existing file.
func (c Config) Write(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(c.JSON()+"\n"), 0o644)
}
