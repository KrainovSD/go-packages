package collab

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFSStoreWriteReadDelete(t *testing.T) {
	var store, err = NewFSStore(fsStoreDir(t), false, testLogger(t))
	if err != nil {
		t.Fatalf("NewFSStore: %v", err)
	}
	store.Write("doc1", []byte("content"))
	if got := store.Read("doc1"); string(got) != "content" {
		t.Fatalf("read = %q, want %q", got, "content")
	}
	store.Delete("doc1")
	if got := store.Read("doc1"); got != nil {
		t.Fatalf("expected nil after delete, got %q", got)
	}
	store.Delete("doc1")
}

func TestFSStoreWriteFsync(t *testing.T) {
	var store, err = NewFSStore(fsStoreDir(t), true, testLogger(t))
	if err != nil {
		t.Fatalf("NewFSStore: %v", err)
	}
	store.Write("doc1", []byte("content"))
	if got := store.Read("doc1"); string(got) != "content" {
		t.Fatalf("read = %q, want %q", got, "content")
	}
	var entries, _ = os.ReadDir(store.path)
	for _, e := range entries {
		if e.Name() != "doc1.bin" {
			t.Fatalf("unexpected leftover file %q", e.Name())
		}
	}
}

func TestFSStoreOverwrite(t *testing.T) {
	var store, err = NewFSStore(fsStoreDir(t), false, testLogger(t))
	if err != nil {
		t.Fatalf("NewFSStore: %v", err)
	}
	store.Write("doc1", []byte("one"))
	store.Write("doc1", []byte("two"))
	if got := store.Read("doc1"); string(got) != "two" {
		t.Fatalf("read = %q, want %q", got, "two")
	}
}

func TestFSStoreReadMissing(t *testing.T) {
	var store, err = NewFSStore(fsStoreDir(t), false, testLogger(t))
	if err != nil {
		t.Fatalf("NewFSStore: %v", err)
	}
	if got := store.Read("missing"); got != nil {
		t.Fatalf("expected nil for missing doc, got %q", got)
	}
}

func TestNewFSStoreValidation(t *testing.T) {
	var cases = []struct {
		name string
		path string
	}{
		{"absolute path", "/tmp/abs-path"},
		{"non-local path", "a/../../b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewFSStore(tc.path, false, testLogger(t)); err == nil {
				t.Fatal("expected error from NewFSStore")
			}
		})
	}
}

func fsStoreDir(t *testing.T) string {
	t.Helper()
	var dir = filepath.Join(".", fmt.Sprintf(".fsstore-test-%d", time.Now().UnixNano()))
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}
