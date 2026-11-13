package tui

import (
	"fmt"
	"path/filepath"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/app"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

// The state of an entry and the actions come from package app, which the GUI shares.
// The aliases keep the TUI's short names; wording and presentation stay here.
type (
	entry = app.Entry
	kind  = app.Kind
)

const (
	kindLinked     = app.KindLinked     // ✓ all good
	kindNotLinked  = app.KindNotLinked  // ○ cloud has it, $HOME has nothing: link
	kindLocalOnly  = app.KindLocalOnly  // ● only in $HOME: adopt
	kindBothExist  = app.KindBothExist  // ● real copy in $HOME and in the cloud: force (backup)
	kindBadLink    = app.KindBadLink    // ✗ broken link or link to elsewhere: force
	kindNotInCloud = app.KindNotInCloud // · nowhere, nothing to do
	kindOther      = app.KindOther      // ? socket, device, error
)

var short = app.Short

// icon, label and hint are the three columns of a row.
func icon(e entry) string {
	switch e.Kind {
	case kindLinked:
		return "✓"
	case kindNotLinked:
		return "○"
	case kindLocalOnly, kindBothExist:
		return "●"
	case kindBadLink:
		return "✗"
	case kindNotInCloud:
		return "·"
	default:
		return "?"
	}
}

func label(e entry) string {
	switch e.Kind {
	case kindLinked:
		return "linked"
	case kindNotLinked:
		return "not linked yet"
	case kindLocalOnly:
		return "only in $HOME"
	case kindBothExist:
		return "local " + e.What + " + cloud"
	case kindBadLink:
		if e.Status.State == linker.BrokenLink {
			return "broken link"
		}
		return "links elsewhere"
	case kindNotInCloud:
		return "not in cloud yet"
	default:
		if e.Status.State == linker.Failed {
			return "error"
		}
		return "something else"
	}
}

// hint is the suggested fix, empty when there's nothing to do.
func hint(e entry) string {
	switch e.Kind {
	case kindNotLinked:
		return "link"
	case kindLocalOnly:
		return "adopt"
	case kindBothExist:
		return "force (backup)"
	case kindBadLink:
		return "force"
	default:
		return ""
	}
}

// details explains the selected entry in a few lines: what's there, what the fix would
// do, and the key that does it (always the last line, it's never cut).
func details(e entry) []string {
	switch e.Kind {
	case kindLinked:
		return []string{
			fmt.Sprintf("%s is a symlink to %s.", short(e.Target), short(e.Source)),
			"Key: d deletes the link, D replaces it with a copy.",
		}
	case kindNotLinked:
		return []string{
			fmt.Sprintf("The cloud has %s, but %s doesn't exist yet.", short(e.Source), short(e.Target)),
			"link: create the symlink.",
			"Key: enter (or l) to link.",
		}
	case kindLocalOnly:
		return []string{
			fmt.Sprintf("%s is a real %s, the cloud has none yet.", short(e.Target), e.What),
			fmt.Sprintf("adopt: move it to %s and link it back.", short(e.Source)),
			"Key: enter (or a) to adopt.",
		}
	case kindBothExist:
		return []string{
			fmt.Sprintf("%s is a real %s, and the cloud has %s too.", short(e.Target), e.What, short(e.Source)),
			fmt.Sprintf("force: rename the local one to %s and link the cloud copy.", filepath.Base(e.Fix.Backup)),
			"Key: enter (or f) to force.",
		}
	case kindBadLink:
		what := "a symlink to"
		if e.Status.State == linker.BrokenLink {
			what = "a broken symlink to"
		}
		return []string{
			fmt.Sprintf("%s is %s %s.", short(e.Target), what, short(e.Status.PointsTo)),
			fmt.Sprintf("force: replace it with a link to %s (%s itself stays).", short(e.Source), short(e.Status.PointsTo)),
			"Key: enter (or f) to force.",
		}
	case kindNotInCloud:
		return []string{
			fmt.Sprintf("Neither %s nor a real file or folder at %s.", short(e.Source), short(e.Target)),
			"Nothing to link. Once the other machine adopted it and Nextcloud synced, it shows up as \"not linked yet\".",
		}
	default:
		if e.Status.Err != nil {
			return []string{fmt.Sprintf("%s: %v", short(e.Target), e.Status.Err)}
		}
		return []string{fmt.Sprintf("%s is neither a file, a folder nor a symlink; it's left alone.", short(e.Target))}
	}
}
