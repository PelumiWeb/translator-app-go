package models

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDescribesEachModelFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tiny.bin", "abc")
	writeFile(t, dir, "base.bin", "hello world")
	writeFile(t, dir, "notes.txt", "not a model")
	if err := os.Mkdir(filepath.Join(dir, "nested.bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	catalog, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := catalog.List()
	want := []Model{
		{
			ID: "base", Version: "b94d27b9934d", Size: 11,
			// The well-known SHA-256 of "hello world".
			SHA256: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
			URL:    "/v1/models/base",
		},
		{
			ID: "tiny", Version: "ba7816bf8f01", Size: 3,
			// The well-known SHA-256 of "abc".
			SHA256: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
			URL:    "/v1/models/tiny",
		},
	}
	if len(got) != len(want) {
		t.Fatalf("List returned %d models, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("model %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The property the app relies on: what the manifest says about a file is
// true of the bytes that Open returns.
func TestManifestMatchesTheBytesServed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.bin", "first model")
	writeFile(t, dir, "b.bin", "a second, longer model")

	catalog, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, m := range catalog.List() {
		f, _, err := catalog.Open(m.ID)
		if err != nil {
			t.Fatalf("Open(%s): %v", m.ID, err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", m.ID, err)
		}

		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != m.SHA256 {
			t.Errorf("%s: served bytes hash to %s, manifest says %s", m.ID, got, m.SHA256)
		}
		if int64(len(data)) != m.Size {
			t.Errorf("%s: served %d bytes, manifest says %d", m.ID, len(data), m.Size)
		}
	}
}

func TestLoadMissingDirectoryGivesEmptyCatalog(t *testing.T) {
	catalog, err := Load(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := catalog.List(); len(got) != 0 {
		t.Errorf("List = %+v, want empty", got)
	}
	if _, _, err := catalog.Open("anything"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open error = %v, want ErrNotFound", err)
	}
}

func TestOpenAndGetUnknownModel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.bin", "x")
	// A file outside the directory that a careless lookup might reach.
	writeFile(t, filepath.Dir(dir), "secret.bin", "x")

	catalog, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, id := range []string{"missing", "../secret", "base.bin", ""} {
		if _, _, err := catalog.Open(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open(%q) error = %v, want ErrNotFound", id, err)
		}
		if _, err := catalog.Get(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) error = %v, want ErrNotFound", id, err)
		}
	}
}
