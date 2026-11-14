package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/app"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
)

// The first-run wizard: where's the cloud folder, which folders should be in it, what
// will happen, set up. Nothing is written before SetupApply.

// cloudSubfolder is where in the cloud folder Home Sync keeps the home folders.
const cloudSubfolder = "LinuxHome"

// CloudOption is a cloud folder the wizard suggests.
type CloudOption struct {
	Name string `json:"name"` // e.g. "Nextcloud"
	Path string `json:"path"` // full path
	Show string `json:"show"` // ~/… for display
}

// SetupFolder is one of the usual home folders, with this system's name for it.
type SetupFolder struct {
	Key    string `json:"key"`    // "documents", "pictures", …: for the icon and the English name
	Item   string `json:"item"`   // the folder's name in the home folder, e.g. "Dokumente"
	Exists bool   `json:"exists"` // it exists on this computer
}

// SetupInfo is what the wizard starts with.
type SetupInfo struct {
	Home    string        `json:"home"` // ~ for display
	Clouds  []CloudOption `json:"clouds"`
	Folders []SetupFolder `json:"folders"`
}

// home is the home folder the wizard works in: HOME_SYNC_HOME for testing, else $HOME.
func (s *SyncService) home() (string, error) {
	if s.homeDir != "" {
		return s.homeDir, nil
	}
	return os.UserHomeDir()
}

// SetupInfo suggests cloud folders and lists the usual home folders.
func (s *SyncService) SetupInfo() (SetupInfo, error) {
	home, err := s.home()
	if err != nil {
		return SetupInfo{}, err
	}
	info := SetupInfo{Home: app.Short(home), Clouds: findClouds(home), Folders: []SetupFolder{}}
	for _, f := range userFolders(home) {
		f.Exists = isDirAt(filepath.Join(home, f.Item))
		info.Folders = append(info.Folders, f)
	}
	return info, nil
}

// findClouds: the Nextcloud client's sync folders first (they can be anywhere), then the
// usual folder names of cloud clients that exist in home.
func findClouds(home string) []CloudOption {
	var out []CloudOption
	add := func(name, path string) {
		path = filepath.Clean(path)
		for _, c := range out {
			if c.Path == path {
				return
			}
		}
		if isDirAt(path) {
			out = append(out, CloudOption{Name: name, Path: path, Show: app.Short(path)})
		}
	}
	for _, p := range nextcloudFolders(filepath.Join(home, ".config", "Nextcloud", "nextcloud.cfg")) {
		add("Nextcloud", p)
	}
	for _, c := range []struct{ name, dir string }{
		{"Nextcloud", "Nextcloud"}, {"ownCloud", "ownCloud"}, {"Dropbox", "Dropbox"},
		{"Seafile", "Seafile"}, {"pCloud", "pCloudDrive"}, {"MEGA", "MEGA"}, {"OneDrive", "OneDrive"},
	} {
		add(c.name, filepath.Join(home, c.dir))
	}
	return out
}

// nextcloudFolders reads the local sync folders from the Nextcloud client's config:
// lines like `0\Folders\1\localPath=/home/kiwi/Nextcloud/`.
func nextcloudFolders(cfgPath string) []string {
	f, err := os.Open(cfgPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), "=")
		if ok && strings.HasSuffix(key, `\localPath`) && filepath.IsAbs(value) {
			out = append(out, value)
		}
	}
	return out
}

// userFolders are the usual home folders under this system's names: from
// ~/.config/user-dirs.dirs (what file managers use, localized, e.g. "Dokumente"), else
// the English names. Downloads and Public are left out: they're per computer.
func userFolders(home string) []SetupFolder {
	keys := []struct{ key, xdg, fallback string }{
		{"desktop", "XDG_DESKTOP_DIR", "Desktop"},
		{"documents", "XDG_DOCUMENTS_DIR", "Documents"},
		{"pictures", "XDG_PICTURES_DIR", "Pictures"},
		{"music", "XDG_MUSIC_DIR", "Music"},
		{"videos", "XDG_VIDEOS_DIR", "Videos"},
		{"templates", "XDG_TEMPLATES_DIR", "Templates"},
	}
	dirs := readUserDirs(filepath.Join(home, ".config", "user-dirs.dirs"), home)
	var out []SetupFolder
	for _, k := range keys {
		item := k.fallback
		if d, ok := dirs[k.xdg]; ok {
			// Only folders directly in home; "$HOME" itself (unset dirs) is skipped
			if rel, err := filepath.Rel(home, d); err == nil && rel != "." && !strings.Contains(rel, string(filepath.Separator)) && !strings.HasPrefix(rel, "..") {
				item = rel
			} else {
				continue
			}
		}
		out = append(out, SetupFolder{Key: k.key, Item: item})
	}
	return out
}

// readUserDirs parses user-dirs.dirs: XDG_DOCUMENTS_DIR="$HOME/Dokumente".
func readUserDirs(path, home string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		value = strings.Trim(value, `"`)
		value = strings.Replace(value, "$HOME", home, 1)
		out[key] = filepath.Clean(value)
	}
	return out
}

// setupConfig is the config the wizard would write: in memory, with full paths for
// checking, and the same with ~ for the file (so it works for the same user anywhere).
func (s *SyncService) setupConfig(cloudRoot string, items []string) (mem, file config.Config, err error) {
	home, err := s.home()
	if err != nil {
		return mem, file, err
	}
	if !filepath.IsAbs(cloudRoot) || !isDirAt(cloudRoot) {
		return mem, file, fmt.Errorf("%s is not a folder", cloudRoot)
	}
	cloudHome := filepath.Join(cloudRoot, cloudSubfolder)
	mem = config.Config{CloudStorageHomeDir: cloudHome, LocalHomeDir: home, SymlinkMap: slices.Clone(items)}
	file = mem
	if rel, err := filepath.Rel(home, cloudHome); err == nil && !strings.HasPrefix(rel, "..") {
		file.CloudStorageHomeDir = "~/" + filepath.ToSlash(rel)
	}
	if home == userHome() {
		file.LocalHomeDir = "~"
	}
	return mem, file, nil
}

// userHome is the real $HOME, to decide whether "~" in the file means the same folder.
func userHome() string {
	h, _ := os.UserHomeDir()
	return h
}

// SetupPreview shows what setting up would do: the same dry run as Fix, for a config that
// isn't written yet. It changes nothing.
func (s *SyncService) SetupPreview(cloudRoot string, items []string) (Plan, error) {
	mem, _, err := s.setupConfig(cloudRoot, items)
	if err != nil {
		return Plan{}, err
	}
	p := app.Preview(app.ActFix, app.Inspect(mem))
	out := Plan{Action: ActionFix, Steps: []Step{}, Changes: p.Count()}
	for _, st := range p.Steps {
		out.Steps = append(out.Steps, toStep(app.ActFix, st))
	}
	return out, nil
}

// SetupApply writes the config and does what SetupPreview showed (checked again first).
// It refuses to replace an existing config.
func (s *SyncService) SetupApply(cloudRoot string, items []string) (Result, error) {
	if _, err := os.Stat(s.path()); err == nil {
		return Result{}, errors.New("Home Sync is already set up")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Result{}, err
	}
	mem, file, err := s.setupConfig(cloudRoot, items)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(mem.CloudStorageHomeDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := file.Write(s.path()); err != nil {
		return Result{}, fmt.Errorf("saving the settings failed: %w", err)
	}
	entries := app.Inspect(mem)
	res := app.Apply(app.Preview(app.ActFix, entries), entries, mem, s.path())
	out := Result{Action: ActionFix, Steps: []Step{}, Failed: res.Failed()}
	for _, st := range res.Steps {
		out.Steps = append(out.Steps, toStep(app.ActFix, st))
	}
	return out, nil
}

// ChooseCloudFolder opens the system's folder dialog for "another cloud folder".
func (s *SyncService) ChooseCloudFolder() (string, error) {
	home, err := s.home()
	if err != nil {
		return "", err
	}
	return application.Get().Dialog.OpenFile().
		SetTitle("Choose your cloud folder").
		SetDirectory(home).
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetButtonText("Choose").
		PromptForSingleSelection()
}
