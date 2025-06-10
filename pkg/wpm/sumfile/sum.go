package sumfile

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

const SumFile = "wpm.sum"

// Sum represents a single line in the wpm.sum file.
type Sum struct {
	Name    string
	Version string
	Hash    string
}

// SumDB holds the parsed content of a wpm.sum file.
type SumDB struct {
	mu   sync.RWMutex
	file string
	sums map[string]map[string]string // map[name]map[version]hash
}

// NewSumDB creates a new SumDB and loads the sum file from the given path.
func NewSumDB(path string) (*SumDB, error) {
	db := &SumDB{
		file: filepath.Join(path, SumFile),
		sums: make(map[string]map[string]string),
	}

	f, err := os.Open(db.file)
	if err != nil {
		if os.IsNotExist(err) {
			return db, nil // File doesn't exist, which is fine.
		}
		return nil, errors.Wrapf(err, "failed to open %s", db.file)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 3 {
			continue // Ignore malformed lines
		}
		if db.sums[parts[0]] == nil {
			db.sums[parts[0]] = make(map[string]string)
		}
		db.sums[parts[0]][parts[1]] = parts[2]
	}

	return db, scanner.Err()
}

// Add adds a new sum to the database.
func (db *SumDB) Add(name, version, hash string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.sums[name] == nil {
		db.sums[name] = make(map[string]string)
	}
	db.sums[name][version] = hash
}

// Get returns the hash for a given package and version.
func (db *SumDB) Get(name, version string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if v, ok := db.sums[name]; ok {
		hash, ok := v[version]
		return hash, ok
	}
	return "", false
}

// Save writes the current state of the database back to the wpm.sum file.
func (db *SumDB) Save() error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var lines []string
	for name, versions := range db.sums {
		for version, hash := range versions {
			lines = append(lines, fmt.Sprintf("%s %s %s", name, version, hash))
		}
	}
	sort.Strings(lines) // For deterministic output

	return os.WriteFile(db.file, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// CalculateHash computes the SHA256 hash of a reader's content and returns it in "sha256:..." format.
func CalculateHash(r io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", hasher.Sum(nil)), nil
}
