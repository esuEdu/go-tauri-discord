package storage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type Disk struct {
	root string
}

func NewDisk(root string) (*Disk, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Disk{root: absolute}, nil
}

func (d *Disk) path(key string) (string, error) {
	if !ValidKey(key) {
		return "", ErrBadKey
	}
	return filepath.Join(d.root, filepath.FromSlash(key)), nil
}

func (d *Disk) Put(_ context.Context, key, _ string, body io.Reader, _ int64) error {
	name, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}

	partial, err := os.CreateTemp(filepath.Dir(name), ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(partial.Name())

	if _, err := io.Copy(partial, body); err != nil {
		partial.Close()
		return err
	}
	if err := partial.Close(); err != nil {
		return err
	}
	return os.Rename(partial.Name(), name)
}

func (d *Disk) Open(_ context.Context, key string) (io.ReadCloser, error) {
	name, err := d.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (d *Disk) Delete(_ context.Context, key string) error {
	name, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
