// Package models is the catalogue of model files the server hands out to
// the app.
package models

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrNotFound = errors.New("models: no such model")

// modelExt is the extension whisper.cpp model files use.
const modelExt = ".bin"

// Model is one entry in the manifest the app downloads.
type Model struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	URL     string `json:"url"`
}

// Catalog knows the model files in one directory.
type Catalog struct {
	// os.Root confines every open to the directory, as in the audio store.
	root   *os.Root
	models []Model           // sorted by id
	files  map[string]string // id -> file name
}

// Load reads dir and hashes every model file in it. The hashes are computed
// once, here, so serving the manifest is a memory read. Restart the server
// after changing the files.
//
// A missing directory is not an error: the server runs with an empty
// catalogue, which is the normal state before any model is downloaded.
func Load(dir string) (*Catalog, error) {
	c := &Catalog{files: make(map[string]string)}

	root, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening models directory: %w", err)
	}
	c.root = root

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing models directory: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != modelExt {
			continue
		}
		size, sum, err := c.hash(name)
		if err != nil {
			return nil, fmt.Errorf("hashing %s: %w", name, err)
		}

		id := strings.TrimSuffix(name, modelExt)
		c.files[id] = name
		c.models = append(c.models, Model{
			ID: id,
			// The version is taken from the content, so replacing a file
			// changes it without anyone having to remember to bump a number.
			Version: sum[:12],
			Size:    size,
			SHA256:  sum,
			URL:     "/v1/models/" + id,
		})
	}
	sort.Slice(c.models, func(i, j int) bool { return c.models[i].ID < c.models[j].ID })
	return c, nil
}

// hash streams the file through SHA-256. io.Copy uses a small fixed buffer,
// so a model of any size is hashed without being loaded into memory.
func (c *Catalog) hash(name string) (size int64, sum string, err error) {
	f, err := c.root.Open(name)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	h := sha256.New()
	size, err = io.Copy(h, f)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(h.Sum(nil)), nil
}

// List returns the manifest entries, sorted by id.
func (c *Catalog) List() []Model {
	// A copy, so a caller cannot change the catalogue's own slice.
	return append([]Model(nil), c.models...)
}

// Get returns one manifest entry, or ErrNotFound.
func (c *Catalog) Get(id string) (Model, error) {
	for _, m := range c.models {
		if m.ID == id {
			return m, nil
		}
	}
	return Model{}, ErrNotFound
}

// Open returns a model's file and when it was last modified. The caller
// closes it. io.ReadSeekCloser is what http.ServeContent needs in order to
// answer range requests: it seeks to the requested offset.
func (c *Catalog) Open(id string) (io.ReadSeekCloser, time.Time, error) {
	name, ok := c.files[id]
	if !ok {
		return nil, time.Time{}, ErrNotFound
	}
	f, err := c.root.Open(name)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("opening model %s: %w", id, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, time.Time{}, fmt.Errorf("reading model %s: %w", id, err)
	}
	return f, info.ModTime(), nil
}
