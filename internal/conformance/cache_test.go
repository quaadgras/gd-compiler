package conformance

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The folder cache holds the objects that a compiler command compiled from the C files of a
// folder (a package, as gd writes them), keyed by a hash of the command, of the folder's
// files, and of the headers that they include (transitively), so that tests (which share
// the packages that they import) compile each package once.
//
// It is safe for concurrent use, by tests and processes: builders of the same entry take
// an exclusive lock on it (flock, released if a process dies), so one builds it while the
// others wait, and an entry is built in a temporary folder, then renamed into place, so
// that it is complete when it exists.
var folderCache = filepath.Join(os.TempDir(), "gd-conformance-cache")

// cFolders returns the folders (below root) with C files, in a fixed order.
func cFolders(root string) ([]string, error) {
	var folders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".c") {
			if dir := filepath.Dir(path); !slices.Contains(folders, dir) {
				folders = append(folders, dir)
			}
		}
		return err
	})
	slices.Sort(folders)
	return folders, err
}

var includeLine = regexp.MustCompile(`(?m)^\s*#\s*include\s*[<"]([^>"]+)[>"]`)

// folderKey hashes the command, and the files that compiling the folder reads: its files,
// and the headers they include (those found in root, or next to the including file).
func folderKey(root, folder string, command []string) (string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return "", err
	}
	var queue []string
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".c") || strings.HasSuffix(entry.Name(), ".h")) {
			queue = append(queue, filepath.Join(folder, entry.Name()))
		}
	}
	read := make(map[string][]byte)
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if _, ok := read[path]; ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		read[path] = data
		for _, m := range includeLine.FindAllSubmatch(data, -1) {
			for _, candidate := range []string{filepath.Join(root, string(m[1])), filepath.Join(filepath.Dir(path), string(m[1]))} {
				if _, err := os.Stat(candidate); err == nil {
					queue = append(queue, candidate)
					break
				}
			}
		}
	}
	paths := make([]string, 0, len(read))
	for path := range read {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	hash := sha256.New()
	fmt.Fprintf(hash, "%q\n", command)
	for _, path := range paths {
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(hash, "%s %d\n", rel, len(read[path])) // (by their path in root, as tests are in different folders)
		hash.Write(read[path])
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// buildFolder runs the compiler on each C file of folder (with root included), from
// the cache when it has, returning the objects it compiled (when compile is set, otherwise
// the compiler only checks the files) and, on failure, the compiler's output.
func buildFolder(t *testing.T, root, folder string, compiler []string, compile bool) ([]string, []byte, error) {
	key, err := folderKey(root, folder, append(append([]string{}, compiler...), fmt.Sprint(compile)))
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(folderCache, key)
	objects := func() ([]string, error) {
		entries, err := os.ReadDir(entry)
		var paths []string
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".o") {
				paths = append(paths, filepath.Join(entry, e.Name()))
			}
		}
		return paths, err
	}
	if _, err := os.Stat(entry); err == nil {
		paths, err := objects()
		return paths, nil, err
	}
	if err := os.MkdirAll(folderCache, 0755); err != nil {
		t.Fatal(err)
	}
	unlock := lockFile(t, entry+".lock")
	defer unlock()
	if _, err := os.Stat(entry); err == nil { // built while this waited for the lock.
		paths, err := objects()
		return paths, nil, err
	}
	tmp, err := os.MkdirTemp(folderCache, key+".*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".c") {
			continue
		}
		args := append(append([]string{}, compiler[1:]...), "-I", root, filepath.Join(folder, e.Name()))
		if compile { // (numbered, as files may have the same name)
			args = append(args, "-c", "-o", filepath.Join(tmp, fmt.Sprintf("%03d-%s.o", i, strings.TrimSuffix(e.Name(), ".c"))))
		}
		if out, err := command(t, folder, time.Minute, compiler[0], args...); err != nil {
			return nil, out, err
		}
	}
	if err := os.Rename(tmp, entry); err != nil {
		t.Fatal(err)
	}
	paths, err := objects()
	return paths, nil, err
}

// lockFile takes an exclusive lock on the file at path (creating it), returning the function
// that releases it.
func lockFile(t *testing.T, path string) func() {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}
}
