//go:build !windows

package silence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func rejectHardLinks(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("cannot verify source link count")
	}
	if stat.Nlink != 1 {
		return fmt.Errorf("hard-linked audio files are not supported for trimming (links: %d)", stat.Nlink)
	}
	return nil
}

func replaceFile(temporaryPath, destinationPath string) error {
	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(destinationPath))
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}
