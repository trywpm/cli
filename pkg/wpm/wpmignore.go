package wpm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/moby/patternmatcher/ignorefile"
)

// ReadWpmIgnore reads the .wpmignore file from the passed path and
// returns the list of paths to exclude
func ReadWpmIgnore(path string) ([]string, error) {
	var excludes []string

	f, err := os.Open(filepath.Join(path, ".wpmignore"))
	switch {
	case os.IsNotExist(err):
		return excludes, nil
	case err != nil:
		return nil, err
	}
	defer f.Close()

	patterns, err := ignorefile.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("error reading .wpmignore: %w", err)
	}
	return patterns, nil
}
