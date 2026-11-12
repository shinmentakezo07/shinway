package misc

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path with data in a single step: the contents are
// written to a temporary file in the target directory, flushed, and renamed
// over the destination. Readers therefore observe either the previous or the
// new contents, never a truncated file.
//
// When the destination already exists its permission bits are preserved;
// otherwise mode is applied. Directories are not created.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	if info, errStat := os.Stat(path); errStat == nil {
		mode = info.Mode().Perm()
	}

	dir := filepath.Dir(path)
	temp, errCreate := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if errCreate != nil {
		return errCreate
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}

	if errChmod := temp.Chmod(mode); errChmod != nil {
		cleanup()
		return errChmod
	}
	if _, errWrite := temp.Write(data); errWrite != nil {
		cleanup()
		return errWrite
	}
	if errSync := temp.Sync(); errSync != nil {
		cleanup()
		return errSync
	}
	if errClose := temp.Close(); errClose != nil {
		_ = os.Remove(tempPath)
		return errClose
	}
	if errRename := os.Rename(tempPath, path); errRename != nil {
		_ = os.Remove(tempPath)
		return errRename
	}
	// Durably record the rename itself; without this the new name can be lost
	// on power failure even though the file contents were synced.
	if dirHandle, errOpen := os.Open(dir); errOpen == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}
