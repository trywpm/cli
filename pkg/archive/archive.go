// Package archive provides helper functions for dealing with archive files.
package archive

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/moby/patternmatcher"
	"github.com/moby/sys/sequential"
)

const (
	regularFileMode      = 0o644
	impliedDirectoryMode = 0o755

	zstdMagicSkippableStart = 0x184D2A50
	zstdMagicSkippableMask  = 0xFFFFFFF0

	zstdMaxWindowSize = uint64(1 << 25) // 32 MB

	maxCompressedSize int64 = 128 * 1024 * 1024 // 128 MB

	maxCompressionRatio int64 = 250
	ratioCheckThreshold int64 = 5 * 1024 * 1024   // 5 MB
	maxDecompressedSize int64 = 512 * 1024 * 1024 // 512 MB
)

var (
	zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}
)

type TarOptions struct {
	IncludeFiles    []string
	ExcludePatterns []string
	Logger          func(format string, args ...any)
}

// breakoutError is used to differentiate errors related to breaking out
// When testing archive breakout in the unit tests, this error is expected
// in order for the test to pass.
type breakoutError error

// isZstd checks if the source byte slice indicates a Zstandard compressed stream.
func isZstd(source []byte) bool {
	if bytes.HasPrefix(source, zstdMagic) {
		// Zstandard frame
		return true
	}
	// skippable frame
	if len(source) < 8 {
		return false
	}
	// magic number from 0x184D2A50 to 0x184D2A5F.
	if binary.LittleEndian.Uint32(source[:4])&zstdMagicSkippableMask == zstdMagicSkippableStart {
		return true
	}
	return false
}

// IsArchivePath checks if the (possibly compressed) file at the given path
// starts with a tar file header.
func IsArchivePath(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	rdr, err := DecompressStream(file)
	if err != nil {
		return false
	}
	defer rdr.Close()

	r := tar.NewReader(rdr)
	_, err = r.Next()
	return err == nil
}

type readCloserWrapper struct {
	io.Reader
	closer func() error
	closed atomic.Bool
}

func (r *readCloserWrapper) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	if r.closer != nil {
		return r.closer()
	}
	return nil
}

var (
	bufioReader256KPool = &sync.Pool{
		New: func() any { return bufio.NewReaderSize(nil, 256*1024) },
	}
)

type bufferedReader struct {
	buf    *bufio.Reader
	closed atomic.Bool
}

func newBufferedReader(r io.Reader) *bufferedReader {
	buf := bufioReader256KPool.Get().(*bufio.Reader)
	buf.Reset(r)
	return &bufferedReader{buf: buf}
}

func (r *bufferedReader) Read(p []byte) (n int, err error) {
	if r.closed.Load() {
		return 0, io.EOF
	}
	n, err = r.buf.Read(p)
	if err == io.EOF {
		r.Close()
	}
	return
}

func (r *bufferedReader) Peek(n int) ([]byte, error) {
	if r.closed.Load() {
		return nil, io.EOF
	}
	return r.buf.Peek(n)
}

func (r *bufferedReader) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	r.buf.Reset(nil)
	bufioReader256KPool.Put(r.buf)
	r.buf = nil
	return nil
}

// DecompressStream decompresses the archive and returns a ReaderCloser with the decompressed archive.
func DecompressStream(archive io.Reader) (io.ReadCloser, error) {
	buf := newBufferedReader(archive)
	bs, err := buf.Peek(10)
	if err != nil && err != io.EOF {
		return nil, err
	}

	// check if the stream is compressed with zstd
	if !isZstd(bs) {
		return nil, fmt.Errorf("unsupported archive format: expected zstd compressed archive")
	}

	zstdReader, err := zstd.NewReader(buf, zstd.WithDecoderMaxWindow(zstdMaxWindowSize))
	if err != nil {
		return nil, err
	}

	return &readCloserWrapper{
		Reader: zstdReader,
		closer: func() error {
			zstdReader.Close()
			return buf.Close()
		},
	}, nil
}

// FileInfoHeader creates a populated Header from fi.
func FileInfoHeader(name string, fi os.FileInfo, link string) (*tar.Header, error) {
	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return nil, err
	}

	var newPerms os.FileMode
	if fi.IsDir() {
		newPerms = impliedDirectoryMode
	} else if fi.Mode().IsRegular() {
		newPerms = regularFileMode
	}

	if newPerms != 0 {
		hdr.Mode = (hdr.Mode &^ int64(os.ModePerm)) | int64(newPerms)
	}

	// Disable gid, uid, uname, gname, access time, change time for portable tar files.
	hdr.Uid = 0
	hdr.Gid = 0
	hdr.Uname = ""
	hdr.Gname = ""
	hdr.Format = tar.FormatPAX
	hdr.AccessTime = time.Time{}
	hdr.ChangeTime = time.Time{}
	hdr.Name = canonicalTarName(name, fi.IsDir())
	hdr.ModTime = hdr.ModTime.Truncate(time.Second)

	return hdr, nil
}

type tarAppender struct {
	TarWriter *tar.Writer
}

func newTarAppender(writer io.Writer) *tarAppender {
	return &tarAppender{
		TarWriter: tar.NewWriter(writer),
	}
}

// canonicalTarName provides a platform-independent and consistent POSIX-style
// path for files and directories to be archived regardless of the platform.
func canonicalTarName(name string, isDir bool) string {
	name = filepath.ToSlash(name)

	// suffix with '/' for directories
	if isDir && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name
}

// addTarFile adds to the tar archive a file from `path` as `name`
func (ta *tarAppender) addTarFile(path, name string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("cannot add %q: symlinks are not supported", path)
	}

	var link string

	hdr, err := FileInfoHeader(name, fi, link)
	if err != nil {
		return err
	}

	originalHdrName := hdr.Name

	if originalHdrName == "." {
		hdr.Name = "package"

		if fi.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}
	} else {
		hdr.Name = filepath.ToSlash(filepath.Join("package", originalHdrName))
	}

	// if it's not a directory and has more than 1 link, it's hard linked
	// and we don't allow hard links in the archive, so return an error
	if !fi.IsDir() && hasHardlinks(fi) {
		return fmt.Errorf("cannot add %q: hard links are not supported", path)
	}

	if err := ta.TarWriter.WriteHeader(hdr); err != nil {
		return err
	}

	if hdr.Typeflag == tar.TypeReg && hdr.Size > 0 {
		// We use sequential file access to avoid depleting the standby list on
		// Windows. On Linux, this equates to a regular os.Open.
		file, err := sequential.Open(path)
		if err != nil {
			return err
		}

		_, err = copyWithBuffer(ta.TarWriter, file)
		file.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func createTarFile(path, extractDir string, hdr *tar.Header, reader io.Reader, options *TarOptions) error {
	switch hdr.Typeflag {
	case tar.TypeDir:
		// Create directory unless it exists as a directory already.
		// In that case we just want to merge the two
		if fi, err := os.Lstat(path); err != nil || !fi.IsDir() {
			if err := os.Mkdir(path, impliedDirectoryMode); err != nil {
				return err
			}
		}

	case tar.TypeReg:
		// Source is regular file. We use sequential file access to avoid depleting
		// the standby list on Windows. On Linux, this equates to a regular os.OpenFile.
		file, err := sequential.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, regularFileMode)
		if err != nil {
			return err
		}
		if _, err := copyWithBuffer(file, io.LimitReader(reader, hdr.Size)); err != nil {
			file.Close()
			return err
		}
		file.Close()

	case tar.TypeLink, tar.TypeSymlink:
		if options != nil && options.Logger != nil {
			if hdr.Typeflag == tar.TypeLink {
				options.Logger("\tskipping hard link: %q -> %q", path, hdr.Linkname)
			} else {
				options.Logger("\tskipping symlink: %q -> %q", path, hdr.Linkname)
			}
		}

		return nil

	case tar.TypeXGlobalHeader:
		return nil

	default:
		return fmt.Errorf("unhandled archive header type %d", hdr.Typeflag)
	}

	mTime := boundTime(hdr.ModTime)
	aTime := boundTime(latestTime(hdr.AccessTime, hdr.ModTime))

	if err := chtimes(path, aTime, mTime); err != nil {
		return err
	}

	return nil
}

// Tar creates an archive from the directory at `path`, only including files whose relative
// paths are included in `options.IncludeFiles` (if non-nil) or not in `options.ExcludePatterns`.
func Tar(srcPath string, options *TarOptions, reporterFn func(fs.FileInfo)) (*Tarballer, error) {
	tb, err := NewTarballer(srcPath, options, reporterFn)
	if err != nil {
		return nil, err
	}
	go tb.Do()
	return tb, nil
}

// Tarballer is a lower-level interface to TarWithOptions which gives the caller
// control over which goroutine the archiving operation executes on.
type Tarballer struct {
	srcPath          string
	options          *TarOptions
	pm               *patternmatcher.PatternMatcher
	pipeReader       *io.PipeReader
	pipeWriter       *io.PipeWriter
	compressWriter   io.WriteCloser
	fileCount        atomic.Int64
	unpackedSize     atomic.Int64
	FileInfoReporter func(fs.FileInfo)
}

// NewTarballer constructs a new tarballer. The arguments are the same as for
// TarWithOptions.
func NewTarballer(srcPath string, options *TarOptions, reporterFn func(fs.FileInfo)) (*Tarballer, error) {
	pm, err := patternmatcher.New(options.ExcludePatterns)
	if err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()

	zstdWriter, err := zstd.NewWriter(pipeWriter, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		pipeReader.Close()
		pipeWriter.Close()
		return nil, fmt.Errorf("failed to create zstd writer: %w", err)
	}

	return &Tarballer{
		// Fix the source path to work with long path names. This is a no-op
		// on platforms other than Windows.
		srcPath:          addLongPathPrefix(srcPath),
		options:          options,
		pm:               pm,
		pipeReader:       pipeReader,
		pipeWriter:       pipeWriter,
		compressWriter:   zstdWriter,
		FileInfoReporter: reporterFn,
	}, nil
}

// Reader returns the reader for the created archive.
func (t *Tarballer) Reader() io.ReadCloser {
	return t.pipeReader
}

// FileCount returns the number of files added to the archive.
func (t *Tarballer) FileCount() int64 {
	return t.fileCount.Load()
}

// UnpackedSize returns the total size of the files added to the archive.
func (t *Tarballer) UnpackedSize() int64 {
	return t.unpackedSize.Load()
}

// Close closes the reader and writer of the Tarballer.
func (t *Tarballer) Close() error {
	return t.pipeReader.Close()
}

// Do performs the archiving operation in the background. The resulting archive
// can be read from t.Reader(). Do should only be called once on each Tarballer
// instance.
func (t *Tarballer) Do() {
	ta := newTarAppender(t.compressWriter)

	var doErr error

	defer func() {
		if err := ta.TarWriter.Close(); err != nil && doErr == nil {
			doErr = fmt.Errorf("failed to close archive writer: %w", err)
		}

		if err := t.compressWriter.Close(); err != nil && doErr == nil {
			doErr = fmt.Errorf("failed to close compression writer: %w", err)
		}

		if doErr != nil {
			t.pipeWriter.CloseWithError(doErr)
		} else {
			t.pipeWriter.Close()
		}
	}()

	stat, err := os.Lstat(t.srcPath)
	if err != nil {
		doErr = fmt.Errorf("unable to read source path %q: %w", t.srcPath, err)
		return
	}

	if !stat.IsDir() {
		doErr = fmt.Errorf("source path %q is not a directory", t.srcPath)
		return
	}

	includeFiles := t.options.IncludeFiles
	if len(includeFiles) == 0 {
		includeFiles = []string{"."}
	}

	seen := make(map[string]bool)

	for _, include := range includeFiles {
		var (
			parentMatchInfo []patternmatcher.MatchInfo
			parentDirs      []string
		)

		walkRoot := filepath.Join(t.srcPath, include)
		doErr = filepath.WalkDir(walkRoot, func(filePath string, f os.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("unable to stat file %q: %w", t.srcPath, err)
			}

			relFilePath, err := filepath.Rel(t.srcPath, filePath)
			if err != nil || (relFilePath == "." && f.IsDir()) {
				// Error getting relative path OR we are looking
				// at the source directory path. Skip in both situations.
				return nil
			}

			skip := false

			// If "include" is an exact match for the current file
			// then even if there's an "excludePatterns" pattern that
			// matches it, don't skip it.
			if include != relFilePath {
				for len(parentDirs) != 0 {
					lastParentDir := parentDirs[len(parentDirs)-1]
					if strings.HasPrefix(relFilePath, lastParentDir+string(os.PathSeparator)) {
						break
					}
					parentDirs = parentDirs[:len(parentDirs)-1]
					parentMatchInfo = parentMatchInfo[:len(parentMatchInfo)-1]
				}

				var matchInfo patternmatcher.MatchInfo
				if len(parentMatchInfo) != 0 {
					skip, matchInfo, err = t.pm.MatchesUsingParentResults(relFilePath, parentMatchInfo[len(parentMatchInfo)-1])
				} else {
					skip, matchInfo, err = t.pm.MatchesUsingParentResults(relFilePath, patternmatcher.MatchInfo{})
				}
				if err != nil {
					return fmt.Errorf("error matching %q: %w", relFilePath, err)
				}

				if f.IsDir() {
					parentDirs = append(parentDirs, relFilePath)
					parentMatchInfo = append(parentMatchInfo, matchInfo)
				}
			}

			if skip {
				// If we want to skip this file and its a directory
				// then we should first check to see if there's an
				// excludes pattern (e.g. !dir/file) that starts with this
				// dir. If so then we can't skip this dir.

				// Its not a dir then so we can just return/skip.
				if !f.IsDir() {
					return nil
				}

				// No exceptions (!...) in patterns so just skip dir
				if !t.pm.Exclusions() {
					return filepath.SkipDir
				}

				dirSlash := relFilePath + string(filepath.Separator)

				for _, pat := range t.pm.Patterns() {
					if !pat.Exclusion() {
						continue
					}
					if strings.HasPrefix(pat.String()+string(filepath.Separator), dirSlash) {
						// found a match - so can't skip this dir
						return nil
					}
				}

				// No matching exclusion dir so just skip dir
				return filepath.SkipDir
			}

			if seen[relFilePath] {
				return nil
			}
			seen[relFilePath] = true

			if err := ta.addTarFile(filePath, relFilePath); err != nil {
				// if pipe is broken, stop writing tar stream to it
				if err == io.ErrClosedPipe {
					return err
				}

				return fmt.Errorf("unable to add file %q to archive: %w", filePath, err)
			}

			fileInfo, err := f.Info()
			if err != nil {
				return fmt.Errorf("unable to get file info for %q: %w", filePath, err)
			}

			if !f.IsDir() {
				t.fileCount.Add(1)
				t.unpackedSize.Add(fileInfo.Size())

				if t.FileInfoReporter != nil {
					t.FileInfoReporter(fileInfo)
				}
			}

			return nil
		})
		if doErr != nil {
			return
		}
	}
}

// Unpack unpacks the decompressedArchive to dest with options.
func Unpack(decompressedArchive io.Reader, dest string, options *TarOptions) error {
	tr := tar.NewReader(decompressedArchive)

	var dirs []*tar.Header
	var totalSize int64

	// Iterate through the files in the archive.
loop:
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			// end of archive
			break
		}
		if err != nil {
			return err
		}

		// ignore XGlobalHeader early to avoid creating parent directories for them
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}

		// Normalize name, for safety and for a simple is-root check
		// This keeps "../" as-is, but normalizes "/../" to "/". Or Windows:
		// This keeps "..\" as-is, but normalizes "\..\" to "\".
		hdr.Name = filepath.Clean(hdr.Name)

		// Check for absolute paths or paths with ".." that would escape the destination directory
		if !filepath.IsLocal(hdr.Name) {
			return breakoutError(fmt.Errorf("invalid archive: insecure path %q (potential directory traversal)", hdr.Name))
		}

		if hdr.Size < 0 {
			return fmt.Errorf("invalid archive: negative size %d for %q", hdr.Size, hdr.Name)
		}

		if hdr.Size > maxDecompressedSize {
			return fmt.Errorf("invalid archive: entry %q declares size %d exceeding %d limit", hdr.Name, hdr.Size, maxDecompressedSize)
		}

		totalSize += hdr.Size
		if totalSize > maxDecompressedSize {
			return fmt.Errorf("invalid archive: total declared size exceeds %d limit", maxDecompressedSize)
		}

		for _, exclude := range options.ExcludePatterns {
			if strings.HasPrefix(filepath.ToSlash(hdr.Name), filepath.ToSlash(exclude)) {
				continue loop
			}
		}

		// Ensure that the parent directory exists.
		err = createImpliedDirectories(dest, hdr)
		if err != nil {
			return err
		}

		path := filepath.Join(dest, hdr.Name)
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return breakoutError(fmt.Errorf("%q is outside of %q", hdr.Name, dest))
		}

		// If path exits we almost always just want to remove and replace it
		// The only exception is when it is a directory *and* the file from
		// the layer is also a directory. Then we want to merge them (i.e.
		// just apply the metadata from the layer).
		if fi, err := os.Lstat(path); err == nil {
			if fi.IsDir() && hdr.Typeflag != tar.TypeDir {
				return fmt.Errorf("cannot overwrite directory %q with non-directory %q", path, dest)
			}

			if !fi.IsDir() && hdr.Typeflag == tar.TypeDir {
				return fmt.Errorf("cannot overwrite non-directory %q with directory %q", path, dest)
			}

			if fi.IsDir() && hdr.Name == "." {
				continue
			}

			if !fi.IsDir() || hdr.Typeflag != tar.TypeDir {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
			}
		}

		if err := createTarFile(path, dest, hdr, tr, options); err != nil {
			return err
		}

		// Directory mtimes must be handled at the end to avoid further
		// file creation in them to modify the directory mtime
		if hdr.Typeflag == tar.TypeDir {
			dirs = append(dirs, hdr)
		}
	}

	for _, hdr := range dirs {
		path := filepath.Join(dest, hdr.Name)

		if err := chtimes(path, boundTime(latestTime(hdr.AccessTime, hdr.ModTime)), boundTime(hdr.ModTime)); err != nil {
			return err
		}
	}
	return nil
}

// createImpliedDirectories will create all parent directories of the current path with default permissions.
func createImpliedDirectories(dest string, hdr *tar.Header) error {
	// Not the root directory, ensure that the parent directory exists
	if !strings.HasSuffix(hdr.Name, string(os.PathSeparator)) {
		parent := filepath.Dir(hdr.Name)
		parentPath := filepath.Join(dest, parent)
		if _, err := os.Lstat(parentPath); err != nil && os.IsNotExist(err) {
			err = os.MkdirAll(parentPath, impliedDirectoryMode)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// readTracker wraps an io.Reader to track the total number of bytes read.
type readTracker struct {
	reader    io.Reader
	bytesRead int64
}

func (t *readTracker) Read(p []byte) (int, error) {
	n, err := t.reader.Read(p)
	t.bytesRead += int64(n)
	return n, err
}

// extractionLimiter wraps the decompressed stream and enforces size and ratio limits.
type extractionLimiter struct {
	decompressedStream io.Reader
	compressedTracker  *readTracker
	decompressedBytes  int64
}

func (b *extractionLimiter) Read(p []byte) (int, error) {
	if b.compressedTracker.bytesRead > maxCompressedSize {
		return 0, fmt.Errorf("invalid archive: compressed size exceeds 256MB limit")
	}
	if b.decompressedBytes >= maxDecompressedSize {
		return 0, fmt.Errorf("invalid archive: decompressed size exceeds 512MB limit (potential zip bomb)")
	}

	remaining := maxDecompressedSize - b.decompressedBytes
	readBuf := p
	if int64(len(readBuf)) > remaining {
		readBuf = readBuf[:remaining]
	}

	n, err := b.decompressedStream.Read(readBuf)
	b.decompressedBytes += int64(n)

	if b.decompressedBytes > ratioCheckThreshold {
		cBytes := b.compressedTracker.bytesRead
		if cBytes == 0 {
			cBytes = 1 // prevent division by zero
		}
		if b.decompressedBytes >= int64(maxCompressionRatio)*cBytes {
			return n, fmt.Errorf("invalid archive: compression ratio exceeds 99.6%% (potential zip bomb)")
		}
	}

	return n, err
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
func Untar(tarArchive io.Reader, dest string, options *TarOptions) error {
	if tarArchive == nil {
		return errors.New("empty archive")
	}

	dest = filepath.Clean(dest)
	if options == nil {
		options = &TarOptions{}
	}
	if options.ExcludePatterns == nil {
		options.ExcludePatterns = []string{}
	}

	compressedTracker := &readTracker{reader: tarArchive}

	decompressedArchive, err := DecompressStream(compressedTracker)
	if err != nil {
		return err
	}
	defer decompressedArchive.Close()

	detector := &extractionLimiter{
		compressedTracker:  compressedTracker,
		decompressedStream: decompressedArchive,
	}

	return Unpack(detector, dest, options)
}
