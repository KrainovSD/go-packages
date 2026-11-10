//go:build cgo

package rocksdb

import (
	"iter"
	"unsafe"
)

const valueLenPrefix = 8

type SingleData struct {
	arena     *Arena
	arenaChan chan *Arena
	merged    []byte
}

func (d *SingleData) Close() {
	d.arena.reset()
	d.arenaChan <- d.arena
}

func (d *SingleData) Read() []byte {
	var cursor = newArenaCursor(d.arena)
	var ptr, ptrLen, merged, ok = cursor.next()
	if !ok {
		return nil
	}
	d.merged = merged
	return viewBytes(ptr, ptrLen)
}

type MultipleData struct {
	returnKeys bool
	keys       [][]byte
	arena      *Arena
	arenaChan  chan *Arena
}

func (d *MultipleData) Close() {
	d.arena.reset()
	d.arenaChan <- d.arena
}

func (d *MultipleData) Iter() iter.Seq2[[]byte, []byte] {
	var cursor = newArenaCursor(d.arena)
	return func(yield func([]byte, []byte) bool) {
		var anchors [][]byte
		var index = 0
		for {
			var key []byte
			if d.returnKeys {
				if len(d.keys) > index {
					key = d.keys[index]
				} else {
					var keyPtr, keyLen, keyMerged, keyOk = cursor.next()
					if !keyOk {
						return
					}
					if keyMerged != nil {
						anchors = append(anchors, keyMerged)
					}
					key = viewBytes(keyPtr, keyLen)
				}
			}
			var ptr, ptrLen, merged, ok = cursor.next()
			if !ok {
				return
			}
			if merged != nil {
				anchors = append(anchors, merged)
			}
			if !yield(key, viewBytes(ptr, ptrLen)) {
				return
			}
			index++
		}
	}
}

type MultipleCFData struct {
	returnKeys bool
	keys       [][]byte
	cfs        []uint8
	arena      *Arena
	arenaChan  chan *Arena
}

type CFData struct {
	Data []byte
	CF   uint8
}

func (d *MultipleCFData) Close() {
	d.arena.reset()
	d.arenaChan <- d.arena
}

func (d *MultipleCFData) Iter() iter.Seq2[[]byte, CFData] {
	var cursor = newArenaCursor(d.arena)
	return func(yield func([]byte, CFData) bool) {
		var anchors [][]byte
		var index = 0
		for {
			var key []byte
			if d.returnKeys && len(d.keys) > index {
				key = d.keys[index]
			}
			var ptr, ptrLen, merged, ok = cursor.next()
			if !ok {
				return
			}
			if merged != nil {
				anchors = append(anchors, merged)
			}
			var cf uint8
			if len(d.cfs) > index {
				cf = d.cfs[index]
			}
			if !yield(key, CFData{Data: viewBytes(ptr, ptrLen), CF: cf}) {
				return
			}
			index++
		}
	}
}

func viewBytes(p unsafe.Pointer, n uint64) []byte {
	if p == nil || n == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(p), int(n))
}
