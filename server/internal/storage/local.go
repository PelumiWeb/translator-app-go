// Package storage keeps uploaded audio until its job has been processed.
package storage

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// Local stores each blob as a file in one directory.
type Local struct {
	// os.Root (Go 1.24+) refuses any path that would leave its directory,
	// including "../" and symlinks, so a bad key cannot reach other files.
	root *os.Root
}

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating audio directory: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening audio directory: %w", err)
	}
	return &Local{root: root}, nil
}

// Put writes r to a new blob named key, replacing any existing one.
func (l *Local) Put(key string, r io.Reader) error {
	f, err := l.root.Create(key)
	if err != nil {
		return fmt.Errorf("creating blob %s: %w", key, err)
	}

	_, copyErr := io.Copy(f, r)
	// Close can report a failed write that Copy did not see (the OS buffers
	// writes), so its error matters for files opened for writing.
	closeErr := f.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		_ = l.root.Remove(key) // do not leave half a file behind
		return fmt.Errorf("writing blob %s: %w", key, err)
	}
	return nil
}

func (l *Local) Open(key string) (io.ReadCloser, error) {
	f, err := l.root.Open(key)
	if err != nil {
		return nil, fmt.Errorf("opening blob %s: %w", key, err)
	}
	return f, nil
}

// Delete removes a blob. Deleting one that does not exist is not an error.
func (l *Local) Delete(key string) error {
	err := l.root.Remove(key)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("deleting blob %s: %w", key, err)
	}
	return nil
}

func (l *Local) Close() error {
	return l.root.Close()
}
