package archive

import (
	"os"
)

func hasHardlinks(fi os.FileInfo) bool {
	return false
}
