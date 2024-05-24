package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func RenameFolders(pattern, newPrefix string) error {
	folders, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, folder := range folders {
		newName := newPrefix
		err := os.Rename(folder, newName)
		if err != nil {
			return err
		}
		fmt.Printf("Renamed %s to %s\n", folder, newName)
	}

	return nil
}
