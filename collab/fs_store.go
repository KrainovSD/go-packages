package collab

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type FSStore struct {
	fsync bool
	path  string
	log   *slog.Logger
}

func NewFSStore(path string, fsync bool, log *slog.Logger) (*FSStore, error) {
	if log == nil {
		return nil, errors.New("logger is required")
	}
	if !filepath.IsLocal(path) {
		return nil, errors.New("invalid path: must be local and relative")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("create state folder: %w", err)
	}
	return &FSStore{path: path, log: log, fsync: fsync}, nil
}

func (s *FSStore) Write(name string, content []byte) {
	if !filepath.IsLocal(name) {
		s.log.Error("write state: name not local path", "name", name)
		return
	}
	var filename = filepath.Join(s.path, name+".bin")
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		s.log.Error("write state: create dir", "error", err, "name", name)
		return
	}
	if s.fsync {
		var tmp *os.File
		var err error
		if tmp, err = os.CreateTemp(s.path, ".tmp-*"); err != nil {
			s.log.Error("write state: create temp", "error", err, "name", name)
			return
		}
		var tmpName = tmp.Name()
		defer func() {
			tmp.Close()
			if tmpName != "" {
				os.Remove(tmpName)
			}
		}()
		if err = tmp.Chmod(0o644); err != nil {
			s.log.Error("write state: cmod", "error", err, "name", name)
			return
		}
		if _, err = tmp.Write(content); err != nil {
			s.log.Error("write state: write", "error", err, "name", name)
			return
		}
		if err = tmp.Sync(); err != nil {
			s.log.Error("write state: sync", "error", err, "name", name)
			return
		}
		if err = tmp.Close(); err != nil {
			s.log.Error("write state: close", "error", err, "name", name)
			return
		}
		if err = os.Rename(tmpName, filename); err != nil {
			s.log.Error("write state: rename", "error", err, "name", name)
			return
		}
		tmpName = ""
	} else {
		var tmp = filename + ".tmp"
		defer func() {
			if tmp != "" {
				os.Remove(tmp)
			}
		}()
		if err := os.WriteFile(tmp, content, 0o644); err != nil {
			s.log.Error("write state: write", "error", err, "name", name)
			return
		}
		if err := os.Rename(tmp, filename); err != nil {
			s.log.Error("write state: rename", "error", err, "name", name)
			return
		}
		tmp = ""
	}
}

func (s *FSStore) Read(name string) []byte {
	if !filepath.IsLocal(name) {
		s.log.Error("read state: name not local path", "name", name)
		return nil
	}
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
	if !filepath.IsLocal(name) {
		s.log.Error("delete state: name not local path", "name", name)
		return
	}
	var err = os.Remove(filepath.Join(s.path, name+".bin"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.log.Error("delete state", "error", err, "name", name)
	}
}

func (s *FSStore) Files() ([]string, error) {
	var result = make([]string, 0)
	var err error
	if err = filepath.WalkDir(s.path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bin") {
			return nil
		}
		var name string
		if name, err = filepath.Rel(s.path, path); err != nil {
			return err
		}
		result = append(result, strings.TrimSuffix(name, ".bin"))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("read states: %w", err)
	}
	return result, nil
}
