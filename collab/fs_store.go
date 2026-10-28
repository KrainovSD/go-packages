package collab

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

type FSStore struct {
	path string
	log  *slog.Logger
}

func NewFSStore(path string, log *slog.Logger) (*FSStore, error) {
	if log == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("create state folder: %w", err)
	}
	return &FSStore{path: path, log: log}, nil
}

func (s *FSStore) Write(name string, content []byte) {
	var filename = filepath.Join(s.path, name+".bin")
	var tmp = filename + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		s.log.Error("write state", "error", err, "name", name)
		return
	}
	if err := os.Rename(tmp, filename); err != nil {
		s.log.Error("write state", "error", err, "name", name)
	}
}

func (s *FSStore) Read(name string) []byte {
	var content, err = os.ReadFile(filepath.Join(s.path, name+".bin"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		s.log.Error("read state", "error", err, "name", name)
		return nil
	}
	return content
}

func (s *FSStore) Delete(name string) {
	var err = os.Remove(filepath.Join(s.path, name+".bin"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.log.Error("delete state", "error", err, "name", name)
	}
}
