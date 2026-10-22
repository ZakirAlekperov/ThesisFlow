//go:build windows

package storagepath

// SyncParent is a deliberate documented no-op on Windows: Go's standard
// os.File.Sync does not provide a portable directory fsync facility there.
// Atomic rename semantics and SQLite's write-ahead durability still vary
// across filesystem and device; do not claim power-loss atomicity.
// RU: На Windows гарантированная fsync каталога здесь недоступна.
func SyncParent(filePath string) error {
	return nil
}
