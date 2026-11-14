package main

import (
	"errors"
	"io/fs"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/app"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

// SyncService is what the frontend calls: every exported method becomes a TypeScript
// function in frontend/bindings (wails3 generate bindings). The logic is package app,
// shared with the TUI; this only turns its results into what the UI shows.
type SyncService struct {
	// configPath is the config file; empty means the default path.
	configPath string
	// homeDir is the home folder the setup wizard works in; empty means $HOME.
	homeDir string
}

// Status is one of the states an entry can be in, as the UI names them.
type Status string

const (
	StatusSynced    Status = "synced"    // linked into the cloud, all good
	StatusReady     Status = "ready"     // in the cloud, not linked here yet
	StatusLocalOnly Status = "localOnly" // only on this computer, not in the cloud yet
	StatusConflict  Status = "conflict"  // on this computer and in the cloud
	StatusBroken    Status = "broken"    // a link that points to the wrong place or nowhere
	StatusMissing   Status = "missing"   // neither here nor in the cloud
	StatusOther     Status = "other"     // something unusual, see Detail
)

// Entry is one folder or file in the list.
type Entry struct {
	Item   string `json:"item"`   // as in the config, e.g. "Documents" or ".config/fish"
	Status Status `json:"status"` // see the Status constants
	IsDir  bool   `json:"isDir"`  // folder or file (for the icon)
	Hidden bool   `json:"hidden"` // a dotfile or inside a dot folder: "advanced"
	Home   string `json:"home"`   // where it is on this computer, e.g. ~/Documents
	Cloud  string `json:"cloud"`  // where it is in the cloud folder
	// PointsTo is where a wrong or broken link points (StatusBroken).
	PointsTo string `json:"pointsTo,omitempty"`
	// Detail explains StatusOther (e.g. an error).
	Detail string `json:"detail,omitempty"`
}

// Overview is everything the main window shows.
type Overview struct {
	ConfigPath string  `json:"configPath"`
	CloudDir   string  `json:"cloudDir"`
	HomeDir    string  `json:"homeDir"`
	Entries    []Entry `json:"entries"`
	// NeedsSetup: there's no config yet (the first-run wizard comes in a later step).
	NeedsSetup bool `json:"needsSetup"`
}

func (s *SyncService) path() string {
	if s.configPath != "" {
		return s.configPath
	}
	return config.DefaultPath()
}

// Overview reads the config and checks every entry. It changes nothing.
func (s *SyncService) Overview() (Overview, error) {
	path := s.path()
	out := Overview{ConfigPath: app.Short(path), Entries: []Entry{}}
	cfg, err := config.Load(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		out.NeedsSetup = true
		return out, nil
	case err != nil:
		return out, err
	}
	out.CloudDir = app.Short(cfg.CloudStorageHomeDir)
	out.HomeDir = app.Short(cfg.LocalHomeDir)
	for _, e := range app.Inspect(cfg) {
		out.Entries = append(out.Entries, toEntry(e))
	}
	return out, nil
}

func toEntry(e app.Entry) Entry {
	out := Entry{
		Item:   e.Item,
		Home:   app.Short(e.Target),
		Cloud:  app.Short(e.Source),
		Hidden: isHidden(e.Item),
		IsDir:  e.What == "folder" || isDirAt(e.Source),
	}
	switch e.Kind {
	case app.KindLinked:
		out.Status = StatusSynced
	case app.KindNotLinked:
		out.Status = StatusReady
	case app.KindLocalOnly:
		out.Status = StatusLocalOnly
	case app.KindBothExist:
		out.Status = StatusConflict
	case app.KindBadLink:
		out.Status = StatusBroken
		out.PointsTo = app.Short(e.Status.PointsTo)
	case app.KindNotInCloud:
		out.Status = StatusMissing
	default:
		out.Status = StatusOther
		if e.Status.State == linker.Failed && e.Status.Err != nil {
			out.Detail = e.Status.Err.Error()
		} else {
			out.Detail = "This is neither a file, a folder nor a link; Home Sync leaves it alone."
		}
	}
	return out
}
