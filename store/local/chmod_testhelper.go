package local

import (
	"os"
	"path/filepath"
)

// ChmodStoreTreeWritable makes imported store objects deletable (tests only).
func ChmodStoreTreeWritable(root string) error {
	storeDir := filepath.Join(root, "store")
	return filepath.WalkDir(storeDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		mode := os.FileMode(0o777)
		if d.IsDir() {
			mode = 0o777
		}
		return os.Chmod(p, mode)
	})
}
