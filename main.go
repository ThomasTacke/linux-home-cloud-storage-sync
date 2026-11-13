// linux-home-cloud-storage-sync keeps a cloud-storage "home" folder linked into $HOME.
package main

import (
	"fmt"
	"os"

	"github.com/ThomasTacke/linux-home-cloud-storage-sync/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
