package util

import (
	"errors"
	"io/fs"
	"os"
)

func DirExists(path string) bool {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return info.IsDir()
}
