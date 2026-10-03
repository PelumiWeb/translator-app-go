package storage

import (
	"io"
	"strings"
	"testing"
)

func newLocal(t *testing.T) *Local {
	t.Helper()
	// t.TempDir is removed automatically when the test ends.
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestLocalPutOpenDelete(t *testing.T) {
	store := newLocal(t)

	if err := store.Put("clip.wav", strings.NewReader("audio bytes")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	f, err := store.Open("clip.wav")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		t.Fatalf("reading blob: %v", err)
	}
	if string(got) != "audio bytes" {
		t.Errorf("content = %q, want %q", got, "audio bytes")
	}

	if err := store.Delete("clip.wav"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Open("clip.wav"); err == nil {
		t.Error("Open after Delete succeeded, want an error")
	}
	if err := store.Delete("clip.wav"); err != nil {
		t.Errorf("second Delete: %v, want nil", err)
	}
}

func TestLocalRejectsKeysOutsideItsDirectory(t *testing.T) {
	store := newLocal(t)

	for _, key := range []string{"../escape.wav", "/etc/passwd", "a/../../escape.wav"} {
		if err := store.Put(key, strings.NewReader("x")); err == nil {
			t.Errorf("Put(%q) succeeded, want an error", key)
		}
		if _, err := store.Open(key); err == nil {
			t.Errorf("Open(%q) succeeded, want an error", key)
		}
	}
}
