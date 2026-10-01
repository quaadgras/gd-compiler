package c99

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/quaadgras/gd-compiler/internal/source"
)

// The cache of the C code of packages (like the Go build cache): a package's C code depends
// only on gd (with its library, embedded in its executable), on the package's source, and
// on its imports (whose generics it may instantiate), so it is reused when they are the
// same. Its folder is $GD_CACHE (by default, gd in the user's cache folder), or there's no
// cache when GD_CACHE=off. Entries are written to a temporary folder, then renamed into
// place, so that concurrent builds see complete entries (or none).

// cacheDir returns the folder of the cache, or "" when there's none.
func cacheDir() string {
	dir := os.Getenv("GD_CACHE")
	switch dir {
	case "off":
		return ""
	case "":
		base, err := os.UserCacheDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(base, "gd")
	}
	return dir
}

var executableHash = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := os.Open(exe)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
})

// packageKey returns the key of the C code of pkg, given the keys of the packages it imports
// (native packages are part of gd), or "" when it can't be cached.
func packageKey(pkg source.Package, keys map[string]string) string {
	exe := executableHash()
	if exe == "" {
		return ""
	}
	h := sha256.New()
	fmt.Fprintf(h, "gd %s\npackage %s %s\n", exe, pkg.Path, pkg.Name)
	var names []string
	for _, file := range pkg.Files {
		names = append(names, file.FileSet.File(file.Location.Node.(*ast.File).FileStart).Name())
	}
	sort.Strings(names)
	for _, name := range names {
		data, _ := os.ReadFile(name) // (files of overlays are part of gd)
		fmt.Fprintf(h, "file %s %d\n", name, len(data))
		h.Write(data)
	}
	for _, imported := range pkg.Imports {
		fmt.Fprintf(h, "import %s %s\n", imported, keys[imported])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func cacheEntry(dir, key string) string { return filepath.Join(dir, key[:2], key) }

// restorePackage copies the C code of the package with the path (its header, and the files
// of its folder) from the cache, reporting whether it was there.
func restorePackage(dir, key, path string) bool {
	entry := cacheEntry(dir, key)
	files, err := os.ReadDir(filepath.Join(entry, "pkg"))
	if err != nil {
		return false
	}
	if err := copyFile(filepath.Join(entry, "pkg.h"), "./.c/go/"+path+".h"); err != nil {
		return false
	}
	if err := os.MkdirAll("./.c/go/"+path, 0755); err != nil {
		return false
	}
	for _, f := range files {
		if err := copyFile(filepath.Join(entry, "pkg", f.Name()), filepath.Join("./.c/go", path, f.Name())); err != nil {
			return false
		}
	}
	now := time.Now()
	os.Chtimes(entry, now, now) // (used, see trimCache)
	return true
}

// storePackage copies the C code of the package with the path into the cache.
func storePackage(dir, key, path string) error {
	entry := cacheEntry(dir, key)
	if _, err := os.Stat(entry); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(entry), 0755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(entry), key+".*.tmp")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := copyFile("./.c/go/"+path+".h", filepath.Join(tmp, "pkg.h")); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(tmp, "pkg"), 0755); err != nil {
		return err
	}
	files, err := os.ReadDir("./.c/go/" + path)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			continue // (other packages, such as math/rand of math)
		}
		if err := copyFile(filepath.Join("./.c/go", path, f.Name()), filepath.Join(tmp, "pkg", f.Name())); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, entry); err != nil && !os.IsExist(err) {
		if _, statErr := os.Stat(entry); statErr != nil { // (unless another build stored it)
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, data, 0644)
}

// trimCache removes the entries that haven't been used for 5 days, at most once a day.
func trimCache(dir string) {
	marker := filepath.Join(dir, "trimmed")
	if info, err := os.Stat(marker); err == nil && time.Since(info.ModTime()) < 24*time.Hour {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	os.WriteFile(marker, nil, 0644)
	old := time.Now().Add(-5 * 24 * time.Hour)
	prefixes, _ := os.ReadDir(dir)
	for _, prefix := range prefixes {
		if !prefix.IsDir() {
			continue
		}
		entries, _ := os.ReadDir(filepath.Join(dir, prefix.Name()))
		for _, e := range entries {
			if info, err := e.Info(); err == nil && info.ModTime().Before(old) {
				os.RemoveAll(filepath.Join(dir, prefix.Name(), e.Name()))
			}
		}
	}
}
