package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrNotFound = errors.New("object not found")

type Store struct {
	root string
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		root = "./data"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Put(key string, r io.Reader) (int64, error) {
	path, err := s.pathFor(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}

func (s *Store) Get(key string) (*os.File, error) {
	path, err := s.pathFor(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (s *Store) Delete(key string) error {
	path, err := s.pathFor(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

func (s *Store) pathFor(key string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimSpace(key))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || strings.HasPrefix(clean, "../") {
		return "", errors.New("invalid object key")
	}
	return filepath.Join(s.root, clean), nil
}

