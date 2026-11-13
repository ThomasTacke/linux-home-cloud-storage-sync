package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui [config]",
		Short: "Interactive view: status, fix, adopt, force, delete, add entries",
		Long: `Opens a full-screen view of the config (default: ` + config.DefaultPath() + `):
every entry with its status and details. Select entries with space and fix, link, adopt,
force or delete them; every action shows its dry run first and only runs after confirming.
n adds a new entry (saved to the config), ? shows all keys.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := configPath(args)
			cfg, err := config.Load(path)
			note := ""
			var invalid *config.ValidationError
			switch {
			case errors.Is(err, fs.ErrNotExist):
				cfg, note = config.Defaults(), "defaults, no config at "+path
			case errors.As(err, &invalid):
				return fmt.Errorf("invalid configuration %q:\n  %s", path, strings.Join(invalid.Problems, "\n  "))
			case err != nil:
				return err
			}
			_, err = tea.NewProgram(tui.New(cfg, path, note)).Run()
			return err
		},
	}
}
