// Package cmd holds the Cobra commands.
package cmd

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Name is the program's name, also used in --version.
const Name = "linux-home-cloud-storage-sync"

// version is set at build time (-ldflags "-X .../cmd.version=v0.2.0"); otherwise it comes
// from the module version (go install ...@v0.2.0), and "dev" for a local build.
var version = ""

// Version returns the program version.
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// NewRootCmd builds the command tree. A function, not package globals, so every test gets
// fresh flags.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           Name,
		Short:         "Keep a cloud-storage home folder linked into your Linux home",
		Long:          "Creates symlinks inside your $HOME that point to a folder in your cloud storage (Nextcloud, Dropbox, ...).\nIt never moves, copies or overwrites anything.",
		Version:       Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate(Name + " {{.Version}}\n")
	root.AddCommand(newCreateSymlinksCmd(), newPrintTemplateConfigCmd(), newWriteDefaultConfigCmd())
	return root
}
