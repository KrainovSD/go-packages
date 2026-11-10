//go:build cgo

package rocksdb

import (
	"bytes"
	"testing"
)

func TestArenaAllocPastChunkEnd(t *testing.T) {
	var arena, err = newArena(100, 3)
	if err != nil {
		t.Fatalf("new arena: %v", err)
	}
	defer arena.destroy()
	arena.append(make([]byte, 300))
	arena.append(make([]byte, 61))

	var p = arena.alloc(16, 8)
	if uintptr(p)%8 != 0 {
		t.Fatalf("alloc must be 8-aligned, got offset %d", uintptr(p)%8)
	}
	for _, chunk := range arena.chunks {
		if chunk.len > chunk.cap {
			t.Fatalf("chunk len %d overflows cap %d", chunk.len, chunk.cap)
		}
	}
	var inBounds = false
	for _, chunk := range arena.chunks {
		var base = uintptr(chunk.p)
		var ptr = uintptr(p)
		if ptr >= base && ptr+16 <= base+chunk.cap {
			inBounds = true
			break
		}
	}
	if !inBounds {
		t.Fatal("alloc returned pointer outside arena chunks")
	}
}

func TestReadValue(t *testing.T) {
	var arena, err = newArena(96, 3)
	if err != nil {
		t.Fatalf("new arena: %v", err)
	}
	defer arena.destroy()

	var first = bytes.Repeat([]byte{7}, 81)
	var second = bytes.Repeat([]byte{5}, 40)
	var third = bytes.Repeat([]byte{9}, 160)
	var fourth = []byte("tail!")

	for _, record := range [][]byte{first, second, third, fourth} {
		if err = arena.appendRecord(record); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}

	var cursor = newArenaCursor(arena)
	var cases = []struct {
		want    []byte
		merged  bool
		comment string
	}{
		{first, false, "zero copy in first chunk"},
		{second, false, "record after padding jump to next chunk"},
		{third, true, "value spanning two chunks gets merged"},
		{fourth, false, "record after merged value"},
	}
	for _, c := range cases {
		var ptr, valueLen, merged, ok = cursor.next()
		if !ok {
			t.Fatalf("%s: readValue returned end of data", c.comment)
		}
		if valueLen != uint64(len(c.want)) {
			t.Fatalf("%s: len %d, want %d", c.comment, valueLen, len(c.want))
		}
		if !bytes.Equal(viewBytes(ptr, valueLen), c.want) {
			t.Fatalf("%s: content mismatch", c.comment)
		}
		if (merged != nil) != c.merged {
			t.Fatalf("%s: merged=%v, want %v", c.comment, merged != nil, c.merged)
		}
	}
	var _, _, _, ok = cursor.next()
	if ok {
		t.Fatal("readValue must stop at end of data")
	}
}
