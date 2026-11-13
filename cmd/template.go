package cmd

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/ThomasTacke/linux-home-cloud-storage-sync/internal/config"
)

func newPrintTemplateConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "print-template-config",
		Short: "Prints the template configuration",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), config.Defaults().JSON())
		},
	}
}

func newWriteDefaultConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "write-default-config [path]",
		Short: "Writes the template configuration",
		Long:  "Writes the template configuration to path (default: " + config.DefaultPath() + "). Never overwrites an existing file.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.DefaultPath()
			if len(args) == 1 {
				path = config.ExpandHome(args[0])
			}
			err := config.Defaults().Write(path)
			switch {
			case errors.Is(err, fs.ErrExist):
				return fmt.Errorf("a configuration file already exists at %q", path)
			case err != nil:
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Default config written to %q\n", path)
			return nil
		},
	}
}
