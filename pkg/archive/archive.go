// Package archive provides helper functions for dealing with archive files.
package archive

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wpm/pkg/log"

	"github.com/cli/safeexec"
	"github.com/moby/patternmatcher"
	"github.com/moby/sys/sequential"
)

type TarOptions struct {
	ShowInfo        bool
	IncludeFiles    []string
	ExcludePatterns []string
}

// breakoutError is used to differentiate errors related to breaking out
// When testing archive breakout in the unit tests, this error is expected
// in order for the test to pass.
type breakoutError error

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
		log.G(context.TODO()).Error("subsequent attempt to close readCloserWrapper")
		if log.GetLevel() >= log.DebugLevel {
			log.G(context.TODO()).Errorf("stack trace: %s", string(debug.Stack()))
		}

		return nil
	}
	if r.closer != nil {
		return r.closer()
	}
	return nil
}

var (
	bufioReader32KPool = &sync.Pool{
		New: func() interface{} { return bufio.NewReaderSize(nil, 32*1024) },
	}
)

type bufferedReader struct {
	buf *bufio.Reader
}

func newBufferedReader(r io.Reader) *bufferedReader {
	buf := bufioReader32KPool.Get().(*bufio.Reader)
	buf.Reset(r)
	return &bufferedReader{buf}
}

func (r *bufferedReader) Read(p []byte) (n int, err error) {
	if r.buf == nil {
		return 0, io.EOF
	}
	n, err = r.buf.Read(p)
	if err == io.EOF {
		r.buf.Reset(nil)
		bufioReader32KPool.Put(r.buf)
		r.buf = nil
	}
	return
}

func (r *bufferedReader) Peek(n int) ([]byte, error) {
	if r.buf == nil {
		return nil, io.EOF
	}
	return r.buf.Peek(n)
}

func gzDecompress(ctx context.Context, buf io.Reader) (io.ReadCloser, error) {
	if noPigzEnv := os.Getenv("WPM_NO_PIGZ"); noPigzEnv != "" {
		noPigz, err := strconv.ParseBool(noPigzEnv)
		if err != nil {
			log.G(ctx).WithError(err).Warn("invalid value in WPM_NO_PIGZ env var")
		}
		if noPigz {
			log.G(ctx).Debugf("use of pigz is disabled due to WPM_NO_PIGZ=%s", noPigzEnv)
			return gzip.NewReader(buf)
		}
	}

	unpigzPath, err := safeexec.LookPath("unpigz")
	if err != nil {
		log.G(ctx).Debugf("unpigz binary not found, falling back to go gzip library")
		return gzip.NewReader(buf)
	}

	log.G(ctx).Debugf("Using %s to decompress", unpigzPath)

	return cmdStream(exec.CommandContext(ctx, unpigzPath, "-d", "-c"), buf)
}

// DecompressStream decompresses the archive and returns a ReaderCloser with the decompressed archive.
func DecompressStream(archive io.Reader) (io.ReadCloser, error) {
	buf := newBufferedReader(archive)
	bs, err := buf.Peek(10)
	if err != nil && err != io.EOF {
		// Note: we'll ignore any io.EOF error because there are some odd
		// cases where the layer.tar file will be empty (zero bytes) and
		// that results in an io.EOF from the Peek() call. So, in those
		// cases we'll just treat it as a non-compressed stream and
		// that means just create an empty layer.
		// See Issue 18170
		return nil, err
	}

	// check gzip header with magic bytes
	if !bytes.HasPrefix(bs, []byte{0x1F, 0x8B, 0x08}) {
		return nil, fmt.Errorf("invalid tarball format")
	}

	ctx, cancel := context.WithCancel(context.Background())
	gzReader, err := gzDecompress(ctx, buf)
	if err != nil {
		cancel()
		return nil, err
	}

	return &readCloserWrapper{
		Reader: gzReader,
		closer: func() error {
			cancel()
			return gzReader.Close()
		},
	}, nil
}

// FileInfoHeader creates a populated Header from fi.
func FileInfoHeader(name string, fi os.FileInfo, link string) (*tar.Header, error) {
	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return nil, err
	}
	hdr.Format = tar.FormatPAX
	hdr.ModTime = hdr.ModTime.Truncate(time.Second)
	hdr.AccessTime = time.Time{}
	hdr.ChangeTime = time.Time{}
	hdr.Mode = int64(chmodTarEntry(os.FileMode(hdr.Mode)))
	hdr.Name = canonicalTarName(name, fi.IsDir())
	return hdr, nil
}

type tarAppender struct {
	TarWriter *tar.Writer

	// for hardlink mapping
	SeenFiles map[uint64]string
}

func newTarAppender(writer io.Writer) *tarAppender {
	return &tarAppender{
		SeenFiles: make(map[uint64]string),
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

	var link string
	if fi.Mode()&os.ModeSymlink != 0 {
		var err error
		link, err = os.Readlink(path)
		if err != nil {
			return err
		}
	}

	hdr, err := FileInfoHeader(name, fi, link)
	if err != nil {
		return err
	}

	// if it's not a directory and has more than 1 link,
	// it's hard linked, so set the type flag accordingly
	if !fi.IsDir() && hasHardlinks(fi) {
		inode, err := getInodeFromStat(fi.Sys())
		if err != nil {
			return err
		}
		// a link should have a name that it links too
		// and that linked name should be first in the tar archive
		if oldpath, ok := ta.SeenFiles[inode]; ok {
			hdr.Typeflag = tar.TypeLink
			hdr.Linkname = oldpath
			hdr.Size = 0 // This Must be here for the writer math to add up!
		} else {
			ta.SeenFiles[inode] = name
		}
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

func createTarFile(path, extractDir string, hdr *tar.Header, reader io.Reader) error {
	// hdr.Mode is in linux format, which we can use for sycalls,
	// but for os.Foo() calls we need the mode converted to os.FileMode,
	// so use hdrInfo.Mode() (they differ for e.g. setuid bits)
	hdrInfo := hdr.FileInfo()

	switch hdr.Typeflag {
	case tar.TypeDir:
		// Create directory unless it exists as a directory already.
		// In that case we just want to merge the two
		if fi, err := os.Lstat(path); !(err == nil && fi.IsDir()) {
			if err := os.Mkdir(path, hdrInfo.Mode()); err != nil {
				return err
			}
		}

	case tar.TypeReg:
		// Source is regular file. We use sequential file access to avoid depleting
		// the standby list on Windows. On Linux, this equates to a regular os.OpenFile.
		file, err := sequential.OpenFile(path, os.O_CREATE|os.O_WRONLY, hdrInfo.Mode())
		if err != nil {
			return err
		}
		if _, err := copyWithBuffer(file, reader); err != nil {
			file.Close()
			return err
		}
		file.Close()

	case tar.TypeLink:
		targetPath := filepath.Join(extractDir, hdr.Linkname)
		// check for hardlink breakout
		if !strings.HasPrefix(targetPath, extractDir) {
			return breakoutError(fmt.Errorf("invalid hardlink %q -> %q", targetPath, hdr.Linkname))
		}
		if err := os.Link(targetPath, path); err != nil {
			return err
		}

	case tar.TypeSymlink:
		// 	path 				-> hdr.Linkname = targetPath
		// e.g. /extractDir/path/to/symlink 	-> ../2/file	= /extractDir/path/2/file
		targetPath := filepath.Join(filepath.Dir(path), hdr.Linkname)

		// the reason we don't need to check symlinks in the path (with FollowSymlinkInScope) is because
		// that symlink would first have to be created, which would be caught earlier, at this very check:
		if !strings.HasPrefix(targetPath, extractDir) {
			return breakoutError(fmt.Errorf("invalid symlink %q -> %q", path, hdr.Linkname))
		}
		if err := os.Symlink(hdr.Linkname, path); err != nil {
			return err
		}

	case tar.TypeXGlobalHeader:
		log.G(context.TODO()).Debug("PAX Global Extended Headers found and ignored")
		return nil

	default:
		return fmt.Errorf("unhandled tar header type %d", hdr.Typeflag)
	}

	aTime := boundTime(latestTime(hdr.AccessTime, hdr.ModTime))
	mTime := boundTime(hdr.ModTime)

	// chtimes doesn't support a NOFOLLOW flag atm
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(hdr.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := chtimes(path, aTime, mTime); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := chtimes(path, aTime, mTime); err != nil {
			return err
		}
	} else {
		if err := lchtimes(path, aTime, mTime); err != nil {
			return err
		}
	}
	return nil
}

// Tar creates an archive from the directory at `path`, only including files whose relative
// paths are included in `options.IncludeFiles` (if non-nil) or not in `options.ExcludePatterns`.
func Tar(srcPath string, options *TarOptions) (io.ReadCloser, error) {
	tb, err := NewTarballer(srcPath, options)
	if err != nil {
		return nil, err
	}
	go tb.Do()
	return tb.Reader(), nil
}

// Convert a tarball to a byte slice
func ToBytes(tarball io.Reader) ([]byte, error) {
	return io.ReadAll(tarball)
}

// Convert to base64 encoded string
func ToBase64(tarball io.Reader) (string, error) {
	bytes, err := ToBytes(tarball)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// Tarballer is a lower-level interface to TarWithOptions which gives the caller
// control over which goroutine the archiving operation executes on.
type Tarballer struct {
	srcPath        string
	options        *TarOptions
	pm             *patternmatcher.PatternMatcher
	pipeReader     *io.PipeReader
	pipeWriter     *io.PipeWriter
	compressWriter io.WriteCloser
}

// NewTarballer constructs a new tarballer. The arguments are the same as for
// TarWithOptions.
func NewTarballer(srcPath string, options *TarOptions) (*Tarballer, error) {
	pm, err := patternmatcher.New(options.ExcludePatterns)
	if err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()

	return &Tarballer{
		// Fix the source path to work with long path names. This is a no-op
		// on platforms other than Windows.
		srcPath:        addLongPathPrefix(srcPath),
		options:        options,
		pm:             pm,
		pipeReader:     pipeReader,
		pipeWriter:     pipeWriter,
		compressWriter: gzip.NewWriter(pipeWriter),
	}, nil
}

// Reader returns the reader for the created archive.
func (t *Tarballer) Reader() io.ReadCloser {
	return t.pipeReader
}

// Do performs the archiving operation in the background. The resulting archive
// can be read from t.Reader(). Do should only be called once on each Tarballer
// instance.
func (t *Tarballer) Do() {
	ta := newTarAppender(
		t.compressWriter,
	)

	defer func() {
		// Make sure to check the error on Close.
		if err := ta.TarWriter.Close(); err != nil {
			log.G(context.TODO()).Errorf("some error occurred while closing tar writer: %s", err)
		}
		if err := t.compressWriter.Close(); err != nil {
			log.G(context.TODO()).Errorf("some error occurred while closing compress writer: %s", err)
		}
		if err := t.pipeWriter.Close(); err != nil {
			log.G(context.TODO()).Errorf("some error occurred while closing pipe writer: %s", err)
		}
	}()

	stat, err := os.Lstat(t.srcPath)
	if err != nil {
		log.G(context.TODO()).Errorf("unable to read source path %s: %s", t.srcPath, err)
		return
	}

	if !stat.IsDir() {
		// We can't later join a non-dir with any includes because the
		// 'walk' will error if "file/." is stat-ed and "file" is not a
		// directory. So, we must split the source path and use the
		// basename as the include.
		if len(t.options.IncludeFiles) > 0 {
			log.G(context.TODO()).Warn("source path is not a directory, include patterns will be ignored")
		}

		dir, base := SplitPathDirEntry(t.srcPath)
		t.srcPath = dir
		t.options.IncludeFiles = []string{base}
	}

	if len(t.options.IncludeFiles) == 0 {
		t.options.IncludeFiles = []string{"."}
	}

	seen := make(map[string]bool)

	for _, include := range t.options.IncludeFiles {
		var (
			parentMatchInfo []patternmatcher.MatchInfo
			parentDirs      []string
		)

		walkRoot := getWalkRoot(t.srcPath, include)
		err = filepath.WalkDir(walkRoot, func(filePath string, f os.DirEntry, err error) error {
			if err != nil {
				log.G(context.TODO()).Errorf("Tar: Can't stat file %s to tar: %s", t.srcPath, err)
				return nil
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
			// matches it, don't skip it. IOW, assume an explicit 'include'
			// is asking for that file no matter what - which is true
			// for some files, like .dockerignore and Dockerfile (sometimes)
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
					log.G(context.TODO()).Errorf("Error matching %s: %v", relFilePath, err)
					return err
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
				log.G(context.TODO()).Errorf("Can't add file %s to tar: %s", filePath, err)
				// if pipe is broken, stop writing tar stream to it
				if err == io.ErrClosedPipe {
					return err
				}
			}

			fileInfo, err := f.Info()
			if err != nil {
				log.G(context.TODO()).Errorf("can't get file info for %s: %s", filePath, err)
				return nil
			}

			if !f.IsDir() && t.options.ShowInfo {
				log.G(context.TODO()).Infof("%.2f kb - %s", float32(fileInfo.Size())/1024, relFilePath)
			}

			return nil
		})
		if err != nil {
			log.G(context.TODO()).Errorf("unable to traverse path %s: %s", t.srcPath, err)
			return
		}
	}
}

// Unpack unpacks the decompressedArchive to dest with options.
func Unpack(decompressedArchive io.Reader, dest string, options *TarOptions) error {
	tr := tar.NewReader(decompressedArchive)

	var dirs []*tar.Header

	// Iterate through the files in the archive.
loop:
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return err
		}

		// ignore XGlobalHeader early to avoid creating parent directories for them
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			log.G(context.TODO()).Debugf("PAX Global Extended Headers found for %s and ignored", hdr.Name)
			continue
		}

		// Normalize name, for safety and for a simple is-root check
		// This keeps "../" as-is, but normalizes "/../" to "/". Or Windows:
		// This keeps "..\" as-is, but normalizes "\..\" to "\".
		hdr.Name = filepath.Clean(hdr.Name)

		for _, exclude := range options.ExcludePatterns {
			if strings.HasPrefix(hdr.Name, exclude) {
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
				// If NoOverwriteDirNonDir is true then we cannot replace
				// an existing directory with a non-directory from the archive.
				return fmt.Errorf("cannot overwrite directory %q with non-directory %q", path, dest)
			}

			if !fi.IsDir() && hdr.Typeflag == tar.TypeDir {
				// If NoOverwriteDirNonDir is true then we cannot replace
				// an existing non-directory with a directory from the archive.
				return fmt.Errorf("cannot overwrite non-directory %q with directory %q", path, dest)
			}

			if fi.IsDir() && hdr.Name == "." {
				continue
			}

			if !(fi.IsDir() && hdr.Typeflag == tar.TypeDir) {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
			}
		}

		if err := createTarFile(path, dest, hdr, tr); err != nil {
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
			err = os.MkdirAll(parentPath, 0777)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
//
// FIXME: specify behavior when target path exists vs. doesn't exist.
func Untar(tarArchive io.Reader, dest string, options *TarOptions) error {
	return untarHandler(tarArchive, dest, options, true)
}

// Handler for teasing out the automatic decompression
func untarHandler(tarArchive io.Reader, dest string, options *TarOptions, decompress bool) error {
	if tarArchive == nil {
		return fmt.Errorf("empty archive")
	}
	dest = filepath.Clean(dest)
	if options == nil {
		options = &TarOptions{}
	}
	if options.ExcludePatterns == nil {
		options.ExcludePatterns = []string{}
	}

	r := tarArchive
	if decompress {
		decompressedArchive, err := DecompressStream(tarArchive)
		if err != nil {
			return err
		}
		defer decompressedArchive.Close()
		r = decompressedArchive
	}

	return Unpack(r, dest, options)
}

// cmdStream executes a command, and returns its stdout as a stream.
// If the command fails to run or doesn't complete successfully, an error
// will be returned, including anything written on stderr.
func cmdStream(cmd *exec.Cmd, input io.Reader) (io.ReadCloser, error) {
	cmd.Stdin = input
	pipeR, pipeW := io.Pipe()
	cmd.Stdout = pipeW
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	// Run the command and return the pipe
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Ensure the command has exited before we clean anything up
	done := make(chan struct{})

	// Copy stdout to the returned pipe
	go func() {
		if err := cmd.Wait(); err != nil {
			pipeW.CloseWithError(fmt.Errorf("%s: %s", err, errBuf.String()))
		} else {
			pipeW.Close()
		}
		close(done)
	}()

	return &readCloserWrapper{
		Reader: pipeR,
		closer: func() error {
			// Close pipeR, and then wait for the command to complete before returning. We have to close pipeR first, as
			// cmd.Wait waits for any non-file stdout/stderr/stdin to close.
			err := pipeR.Close()
			<-done
			return err
		},
	}, nil
}
