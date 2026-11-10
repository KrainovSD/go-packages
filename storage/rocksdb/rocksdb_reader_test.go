//go:build cgo

package rocksdb

import (
	"bytes"
	"testing"
)

func TestIterKeys(t *testing.T) {
	var arena, err = newArena(96, 3)
	if err != nil {
		t.Fatalf("new arena: %v", err)
	}
	defer arena.destroy()

	var records = [][]byte{
		bytes.Repeat([]byte{7}, 81),
		bytes.Repeat([]byte{5}, 40),
		bytes.Repeat([]byte{9}, 160),
		[]byte("tail!"),
	}
	for _, record := range records {
		if err = arena.appendRecord(record); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}

	var data = &MultipleData{returnKeys: true, arena: arena, arenaChan: make(chan *Arena, 1)}
	var keys, values [][]byte
	for key, value := range data.Iter() {
		keys = append(keys, key)
		values = append(values, value)
	}
	if len(keys) != 2 || len(values) != 2 {
		t.Fatalf("got %d pairs, want 2", len(keys))
	}
	for i, wantKey := range [][]byte{records[0], records[2]} {
		if !bytes.Equal(keys[i], wantKey) {
			t.Fatalf("key %d mismatch", i)
		}
	}
	for i, wantValue := range [][]byte{records[1], records[3]} {
		if !bytes.Equal(values[i], wantValue) {
			t.Fatalf("value %d mismatch", i)
		}
	}
}

func TestIterValuesOnly(t *testing.T) {
	var arena, err = newArena(96, 3)
	if err != nil {
		t.Fatalf("new arena: %v", err)
	}
	defer arena.destroy()

	var records = [][]byte{
		bytes.Repeat([]byte{7}, 81),
		bytes.Repeat([]byte{9}, 160),
		[]byte("tail!"),
	}
	for _, record := range records {
		if err = arena.appendRecord(record); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}

	var data = &MultipleData{returnKeys: false, arena: arena, arenaChan: make(chan *Arena, 1)}
	var values [][]byte
	for _, value := range data.Iter() {
		values = append(values, value)
	}
	if len(values) != len(records) {
		t.Fatalf("got %d values, want %d", len(values), len(records))
	}
	for i, want := range records {
		if !bytes.Equal(values[i], want) {
			t.Fatalf("value %d mismatch", i)
		}
	}
}

func TestSingleDataReadEmpty(t *testing.T) {
	var arena, err = newArena(96, 3)
	if err != nil {
		t.Fatalf("new arena: %v", err)
	}
	defer arena.destroy()
	var data = &SingleData{
		arena:     arena,
		arenaChan: make(chan *Arena, 1),
	}
	if v := data.Read(); v != nil {
		t.Fatalf("read on empty arena must return nil, got %d bytes", len(v))
	}
}
