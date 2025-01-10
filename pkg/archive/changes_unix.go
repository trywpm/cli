//go:build !windows

package archive

import (
	"os"
	"syscall"
)

func hasHardlinks(fi os.FileInfo) bool {
	return fi.Sys().(*syscall.Stat_t).Nlink > 1
}
