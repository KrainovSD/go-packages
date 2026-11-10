//go:build cgo

package rocksdb

/*
#cgo CFLAGS: -I./connector
#include "arena.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"encoding/binary"
	"errors"
	"fmt"
	"unsafe"
)

type Arena struct {
	p       *C.db_arena_t
	current uint16
	chunks  []*Chunk
}

const defaultSize = 2 << 20
const defaulTTL = 5

func newArena(initialSize uint64, ttl uint8) (*Arena, error) {
	var cErr CError
	if initialSize == 0 {
		initialSize = defaultSize
	}
	if ttl == 0 {
		ttl = defaulTTL
	}
	var allocSize = (uintptr(initialSize) + 7) & ^uintptr(7)
	var arena = C.db_arena_create(C.size_t(allocSize), C.size_t(ttl), &cErr.p)
	if err := cErr.Error(); err != nil {
		return nil, fmt.Errorf("create arena: %w", err)
	}
	var chunks = make([]*Chunk, 0)
	chunks = append(chunks, newChunk(allocSize))
	return &Arena{
		p:      arena,
		chunks: chunks,
	}, nil
}

func (a *Arena) nextChunk(size int) *Chunk {
	if int(a.current) < len(a.chunks)-1 {
		a.current++
		return a.chunks[a.current]
	}
	var last = a.chunks[len(a.chunks)-1]
	var next = int(last.cap * 2)
	if next < size {
		next = size + 64
	}
	a.chunks = append(a.chunks, newChunk(uintptr(next)))
	a.current = uint16(len(a.chunks) - 1)
	return a.chunks[a.current]
}

func (a *Arena) append(b []byte) (*C.char, C.size_t) {
	if len(b) == 0 {
		return nil, 0
	}
	var chunk = a.chunks[a.current]
	for int(chunk.cap)-int(chunk.len) < len(b) {
		chunk = a.nextChunk(len(b))
	}
	var ptr = (*C.char)(unsafe.Add(chunk.p, chunk.len))
	chunk.len += uintptr(len(b))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(b)), b)
	return ptr, C.size_t(uint64(len(b)))
}

func (a *Arena) alloc(size, align uintptr) unsafe.Pointer {
	var chunk = a.chunks[a.current]
	var offset = (chunk.len + align - 1) &^ (align - 1)
	for offset+size > chunk.cap {
		chunk = a.nextChunk(int(size))
		offset = (chunk.len + align - 1) &^ (align - 1)
	}
	chunk.len = offset + size
	return unsafe.Add(chunk.p, offset)
}

func (a *Arena) cString(s string) *C.char {
	var b = append([]byte(s), 0)
	var p, _ = a.append(b)
	return p
}

func (a *Arena) appendRecord(b []byte) error {
	var cError CError
	var p *C.char
	if len(b) > 0 {
		p = (*C.char)(unsafe.Pointer(&b[0]))
	}
	C.db_arena_append(a.p, p, C.size_t(uint64(len(b))), &cError.p)
	if err := cError.Error(); err != nil {
		return fmt.Errorf("append record: %w", err)
	}
	return nil
}

func (a *Arena) reset() {
	for i := range len(a.chunks) {
		a.chunks[i].len = 0
	}
	a.current = 0
}

func (a *Arena) destroy() {
	C.db_arena_destroy(a.p)
	for i := range len(a.chunks) {
		C.free(a.chunks[i].p)
	}
}

type Chunk struct {
	p   unsafe.Pointer
	len uintptr
	cap uintptr
}

func newChunk(size uintptr) *Chunk {
	var p = C.malloc(C.size_t(size))
	if p == nil {
		panic("arena chunk: out of memory")
	}
	return &Chunk{
		p:   p,
		cap: size,
	}
}

type CError struct {
	p *C.char
}

func (e *CError) Error() error {
	if e.p != nil {
		var err = errors.New(C.GoString(e.p))
		C.free(unsafe.Pointer(e.p))
		return err
	}
	return nil
}

type arenaCursor struct {
	arena  *Arena
	chunk  int
	offset uint64
}

func newArenaCursor(arena *Arena) *arenaCursor {
	return &arenaCursor{
		arena: arena,
	}
}

func (c *arenaCursor) next() (unsafe.Pointer, uint64, []byte, bool) {
	var chunks = unsafe.Slice(c.arena.p.chunks, int(c.arena.p.cap))
	var chunk = chunks[c.chunk]
	if c.offset+valueLenPrefix > uint64(chunk.len) {
		c.chunk++
		if c.chunk >= len(chunks) || chunks[c.chunk].len == 0 {
			return nil, 0, nil, false
		}
		chunk = chunks[c.chunk]
		c.offset = 0
	}
	var raw = unsafe.Slice((*byte)(unsafe.Pointer(chunk.data)), int(chunk.len))
	var valueLen = binary.LittleEndian.Uint64(raw[c.offset : c.offset+valueLenPrefix])
	c.offset += valueLenPrefix
	var avail = uint64(chunk.len) - c.offset
	if avail >= valueLen {
		var ptr = unsafe.Add(unsafe.Pointer(chunk.data), uintptr(c.offset))
		c.offset += valueLen
		return ptr, valueLen, nil, true
	}
	var merged = make([]byte, valueLen)
	copy(merged, raw[c.offset:])
	var copied = avail
	var chunkIndex = c.chunk + 1
	for copied < valueLen {
		var next = chunks[chunkIndex]
		var take = uint64(next.len)
		if copied+take > valueLen {
			take = valueLen - copied
		}
		copy(merged[copied:copied+take], unsafe.Slice((*byte)(unsafe.Pointer(next.data)), int(next.len))[:take])
		copied += take
		c.chunk, c.offset = chunkIndex, take
		chunkIndex++
	}
	return unsafe.Pointer(&merged[0]), valueLen, merged, true
}
