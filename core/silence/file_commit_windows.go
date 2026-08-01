//go:build windows

package silence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func rejectHardLinks(info os.FileInfo) error {
	path := filepath.Clean(info.Name())
	if path == "" {
		return errors.New("cannot verify source link count")
	}
	// os.FileInfo does not retain the complete Windows path. Callers on
	// Windows therefore use validateSourceLinks, which opens the path itself.
	return nil
}

func validateSourceLinks(path string) error {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return fmt.Errorf("open source to verify links: %w", err)
	}
	defer windows.CloseHandle(handle)
	var details windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &details); err != nil {
		return fmt.Errorf("verify source links: %w", err)
	}
	if details.NumberOfLinks != 1 {
		return fmt.Errorf("hard-linked audio files are not supported for trimming (links: %d)", details.NumberOfLinks)
	}
	return nil
}

func replaceFile(temporaryPath, destinationPath string) error {
	from, err := windows.UTF16PtrFromString(temporaryPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destinationPath)
	if err != nil {
		return err
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return err
	}
	return nil
}

func syncDirectory(string) error { return nil }
