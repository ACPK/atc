package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type DirectoryFlag string

func (dir *DirectoryFlag) UnmarshalFlag(value string) error {
	stat, err := os.Stat(value)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		*dir = DirectoryFlag(filepath.Abs(stat.Name()))
	} else {
		return fmt.Errorf("path '%s' is not a directory", value)
	}

	return nil
}
