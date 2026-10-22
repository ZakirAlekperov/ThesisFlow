//go:build !windows

package storagepath

import (
	"fmt"
	"os"
	"path/filepath"
)

// SyncParent fsyncs the directory that contains a newly renamed document.
// This helps preserve rename metadata across process/power interruptions on
// supported Unix filesystems; it is not a guarantee on every storage device.
// RU: Синхронизируем каталог после rename на macOS/Linux.
func SyncParent(filePath string) error {
	dir, err := os.Open(filepath.Dir(filePath))
	if err != nil {
		return fmt.Errorf("open parent directory for sync: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync parent directory: %w", err)
	}
	return nil
}
