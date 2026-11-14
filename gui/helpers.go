package main

import (
	"os"
	"strings"
)

// isHidden: dotfiles and anything inside a dot folder are for advanced users.
func isHidden(item string) bool {
	for _, part := range strings.Split(item, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func isDirAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
