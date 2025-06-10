//go:build !windows

package archive

import (
	"syscall"
)

// addLongPathPrefix adds the Windows long path prefix to the path provided if
// it does not already have it. It is a no-op on platforms other than Windows.
func addLongPathPrefix(srcPath string) string {
	return srcPath
}

func getInodeFromStat(stat interface{}) (inode uint64, err error) {
	s, ok := stat.(*syscall.Stat_t)

	if ok {
		inode = s.Ino
	}

	return
}
