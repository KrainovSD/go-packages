//go:build cgo

package rocksdb

/*
#cgo LDFLAGS: -L./connector/rocksdb_vendor -lrocksdb
#cgo LDFLAGS: -lstdc++ -lm -lpthread -ldl -lsnappy -lz -lbz2 -llz4 -lzstd -luring
#cgo CFLAGS: -I./connector
#include "db.h"
#include "read.h"
#include "write.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/KrainovSD/go-packages/helpers"
)

type DB struct {
	arenaChan chan *Arena
	db        *C.db_t
	idMutexes *helpers.KeyedMutex[uint32]
	arenaSize uint64
	arenaTTL  uint8
}

func NewRocksDB(ctx context.Context, opts DatabaseOptions) (*DB, error) {
	var err error
	if err = opts.validate(); err != nil {
		return nil, fmt.Errorf("options validate: %w", err)
	}
	var arenaCount = runtime.NumCPU() * 3
	var arenaChan = make(chan *Arena, arenaCount)
	for range arenaCount {
		var arena, err = newArena(opts.ArenaIntitialSize, opts.ArenaTTL)
		if err != nil {
			for range len(arenaChan) {
				(<-arenaChan).destroy()
			}
			return nil, fmt.Errorf("new arena: %w", err)
		}
		arenaChan <- arena
	}
	var arena = <-arenaChan
	var cpath = C.CString(opts.Path)
	defer C.free(unsafe.Pointer(cpath))
	var cErr CError
	var db = C.db_open(cpath, opts.Rocks.toC(arena), &cErr.p)
	arena.reset()
	arenaChan <- arena
	if err = cErr.Error(); err != nil {
		C.db_close(db)
		for range cap(arenaChan) {
			(<-arenaChan).destroy()
		}
		return nil, fmt.Errorf("open db: %w", err)
	}
	return &DB{
		arenaChan: arenaChan,
		db:        db,
		idMutexes: helpers.NewKeyedMutex[uint32](),
		arenaSize: opts.ArenaIntitialSize,
		arenaTTL:  opts.ArenaTTL,
	}, nil
}

func (db *DB) lockDB() func() {
	var arenas = make([]*Arena, 0, cap(db.arenaChan))
	for range cap(db.arenaChan) {
		arenas = append(arenas, <-db.arenaChan)
	}
	return func() {
		for _, arena := range arenas {
			db.arenaChan <- arena
		}
	}
}

func (db *DB) Close() {
	C.db_close(db.db)
	for range cap(db.arenaChan) {
		(<-db.arenaChan).destroy()
	}
}

func (db *DB) Read(cf uint8, reader uint8, key []byte) (*SingleData, error) {
	var arena = <-db.arenaChan
	var cKey, CKeyLen = arena.append(key)
	var CValueLen C.size_t
	var cErr CError
	C.db_read(db.db, C.uint8_t(cf), C.uint8_t(reader), cKey, CKeyLen, &CValueLen, arena.p, &cErr.p)
	if err := cErr.Error(); err != nil {
		arena.reset()
		db.arenaChan <- arena
		return nil, err
	}
	return &SingleData{
		arena:     arena,
		arenaChan: db.arenaChan,
	}, nil
}

func (db *DB) ReadAll(opts *ReadOptions) (*MultipleData, error) {
	var arena = <-db.arenaChan
	var cPrefix, cPrefixLen = arena.append(opts.Prefix)
	if opts.PrefixLen != 0 {
		cPrefixLen = C.size_t(opts.PrefixLen)
	}
	var cCursor, cCursorLen = arena.append(opts.Cursor)
	var cOpts = C.db_read_all_opts{
		prefix:       cPrefix,
		prefix_len:   cPrefixLen,
		cf_index:     C.uint8_t(opts.CF),
		reader_index: C.uint8_t(opts.Reader),
		limit:        C.uint64_t(opts.Limit),
		offset:       C.uint64_t(opts.Offset),
		cursor:       cCursor,
		cursor_len:   cCursorLen,
		reverse:      C.bool(opts.Reverse),
		keys:         C.bool(opts.ReturnKeys),
	}
	var cErr CError
	C.db_read_all(db.db, &cOpts, arena.p, &cErr.p)
	if err := cErr.Error(); err != nil {
		arena.reset()
		db.arenaChan <- arena
		return nil, err
	}
	return &MultipleData{
		returnKeys: opts.ReturnKeys,
		arena:      arena,
		arenaChan:  db.arenaChan,
	}, nil
}

func (db *DB) ReadBatchSingle(opts *ReadBatchSingleOptions) (*MultipleData, error) {
	var arena = <-db.arenaChan
	var cKeys = (*C.rocksdb_slice_t)(arena.alloc(
		uintptr(len(opts.Keys))*unsafe.Sizeof(C.rocksdb_slice_t{}),
		unsafe.Alignof(C.rocksdb_slice_t{}),
	))
	var keySlices = unsafe.Slice(cKeys, len(opts.Keys))
	for i := range opts.Keys {
		var data, size = arena.append(opts.Keys[i])
		keySlices[i] = C.rocksdb_slice_t{data: data, size: size}
	}
	var cErr CError
	C.db_read_batch_single_cf(db.db, C.uint8_t(opts.Cf), C.uint8_t(opts.Reader), cKeys,
		C.size_t(uint64(len(opts.Keys))), C.bool(opts.SortedKeys), arena.p, &cErr.p)
	if err := cErr.Error(); err != nil {
		arena.reset()
		db.arenaChan <- arena
		return nil, err
	}
	return &MultipleData{
		returnKeys: true,
		arena:      arena,
		arenaChan:  db.arenaChan,
		keys:       opts.Keys,
	}, nil
}

func (db *DB) ReadBatchMultiple(opts *ReadBatchMultipleOptions) (*MultipleCFData, error) {
	var arena = <-db.arenaChan
	var cCfs = (*C.uint8_t)(arena.alloc(uintptr(len(opts.Keys)), 1))
	var cKeys = (**C.char)(arena.alloc(
		uintptr(len(opts.Keys))*unsafe.Sizeof((*C.char)(nil)),
		unsafe.Alignof((*C.char)(nil)),
	))
	var cKeyLens = (*C.size_t)(arena.alloc(
		uintptr(len(opts.Keys))*unsafe.Sizeof(C.size_t(0)),
		unsafe.Sizeof(C.size_t(0)),
	))
	var cfs = unsafe.Slice(cCfs, len(opts.Keys))
	var keys = unsafe.Slice(cKeys, len(opts.Keys))
	var keyLens = unsafe.Slice(cKeyLens, len(opts.Keys))
	for i := range opts.Keys {
		keys[i], keyLens[i] = arena.append(opts.Keys[i])
		cfs[i] = C.uint8_t(opts.CFs[i])
	}
	var cErr CError
	C.db_read_batch_multi_cf(db.db, cCfs, C.uint8_t(opts.Reader), cKeys, cKeyLens,
		C.size_t(uint64(len(opts.Keys))), arena.p, &cErr.p)
	if err := cErr.Error(); err != nil {
		arena.reset()
		db.arenaChan <- arena
		return nil, err
	}
	return &MultipleCFData{
		returnKeys: true,
		arena:      arena,
		arenaChan:  db.arenaChan,
		keys:       opts.Keys,
		cfs:        opts.CFs,
	}, nil
}

func (db *DB) Write(cf uint8, writer uint8, operation Operation, key []byte, value []byte) error {
	var arena = <-db.arenaChan
	var cKey, cKeyLen = arena.append(key)
	var cValue, cValueLen = arena.append(value)
	var cErr CError
	C.db_write(db.db, C.uint8_t(cf), C.uint8_t(writer), C.uint8_t(operation),
		cKey, cKeyLen, cValue, cValueLen, &cErr.p)
	var err = cErr.Error()
	arena.reset()
	db.arenaChan <- arena
	return err
}

func (db *DB) WriteBatch(writer uint8, opts []WriteBatchOptions) error {
	var arena = <-db.arenaChan
	var err = db.writeBatch(writer, opts, arena)
	arena.reset()
	db.arenaChan <- arena
	return err
}

func (db *DB) writeBatch(writer uint8, opts []WriteBatchOptions, arena *Arena) error {
	var cItems = (**C.db_write_batch_opts_t)(arena.alloc(
		uintptr(len(opts))*unsafe.Sizeof((*C.db_write_batch_opts_t)(nil)),
		unsafe.Alignof((*C.db_write_batch_opts_t)(nil)),
	))
	var items = unsafe.Slice(cItems, len(opts))
	for i := range opts {
		var key, keyLen = arena.append(opts[i].Key)
		var value, valueLen = arena.append(opts[i].Value)
		var reader *C.uint8_t
		if opts[i].Reader != nil {
			reader = (*C.uint8_t)(arena.alloc(1, 1))
			*reader = C.uint8_t(*opts[i].Reader)
		}
		var item = (*C.db_write_batch_opts_t)(arena.alloc(
			unsafe.Sizeof(C.db_write_batch_opts_t{}),
			unsafe.Alignof(C.db_write_batch_opts_t{}),
		))
		*item = C.db_write_batch_opts_t{
			key:          key,
			key_len:      keyLen,
			value:        value,
			value_len:    valueLen,
			cf_index:     C.uint8_t(opts[i].CF),
			reader_index: reader,
			operation:    C.uint8_t(opts[i].Operation),
		}
		items[i] = item
	}
	var cErr CError
	C.db_write_batch(db.db, C.uint8_t(writer), cItems, C.size_t(uint64(len(opts))), &cErr.p)
	return cErr.Error()
}

func (db *DB) ReadDump(reader uint8) ([]byte, error) {
	var dumps = make([]Dump, 0, int(db.db.cf_count))
	for cf := uint8(0); uint64(cf) < uint64(db.db.cf_count); cf++ {
		var dump = Dump{CF: cf}
		var data, err = db.ReadAll(&ReadOptions{CF: cf, Reader: reader, ReturnKeys: true})
		if err != nil {
			return nil, fmt.Errorf("read all cf %d: %w", cf, err)
		}
		for key, value := range data.Iter() {
			dump.Keys = append(dump.Keys, append([]byte(nil), key...))
			dump.Values = append(dump.Values, append([]byte(nil), value...))
		}
		data.Close()
		dumps = append(dumps, dump)
	}
	var raw []byte
	var err error
	if raw, err = json.Marshal(dumps); err != nil {
		return nil, fmt.Errorf("marshal dump: %w", err)
	}
	return raw, nil
}

func (db *DB) WriteDump(writer uint8, dump []byte, force bool) error {
	var dumps []Dump
	var err error
	if err = json.Unmarshal(dump, &dumps); err != nil {
		return fmt.Errorf("unmarshal dump: %w", err)
	}
	var opts = make([]WriteBatchOptions, 0)
	for _, cfDump := range dumps {
		if uint64(cfDump.CF) >= uint64(db.db.cf_count) {
			return fmt.Errorf("dump cf %d is out of range", cfDump.CF)
		}
		for i, key := range cfDump.Keys {
			var value []byte
			if i < len(cfDump.Values) {
				value = cfDump.Values[i]
			}
			opts = append(opts, WriteBatchOptions{
				Key:       key,
				Value:     value,
				CF:        cfDump.CF,
				Operation: OPERATION_PUT,
			})
		}
	}
	if !force {
		if len(opts) > 0 {
			return db.WriteBatch(writer, opts)
		}
		return nil
	}
	var unlock = db.lockDB()
	defer unlock()
	for cf := uint8(1); uint64(cf) < uint64(db.db.cf_count); cf++ {
		var cErr CError
		C.db_cf_drop_and_recreate(db.db, C.uint8_t(cf), &cErr.p)
		if err := cErr.Error(); err != nil {
			return fmt.Errorf("drop and recreate cf %d: %w", cf, err)
		}
	}
	if len(opts) == 0 {
		return nil
	}
	var arena *Arena
	if arena, err = newArena(db.arenaSize, db.arenaTTL); err != nil {
		return fmt.Errorf("new arena: %w", err)
	}
	defer arena.destroy()
	return db.writeBatch(writer, opts, arena)
}

func (db *DB) AutoIncrement(cf uint8, reader uint8, writer uint8, key []byte) (uint64, error) {
	var lockKey = crc32.ChecksumIEEE(key)
	db.idMutexes.Lock(lockKey)
	defer db.idMutexes.Unlock(lockKey)
	var arena = <-db.arenaChan
	var cKey, cKeyLen = arena.append(key)
	var cNext C.uint64_t
	var cErr CError
	C.db_auto_increment(db.db, C.uint8_t(cf), C.uint8_t(reader), C.uint8_t(writer),
		cKey, cKeyLen, &cNext, &cErr.p)
	var err = cErr.Error()
	arena.reset()
	db.arenaChan <- arena
	if err != nil {
		return 0, fmt.Errorf("auto increment cf %d: %w", cf, err)
	}
	return uint64(cNext), nil
}

func (db *DB) DropAndRecreateCF(cf uint8) error {
	var unlock = db.lockDB()
	defer unlock()
	var cErr CError
	C.db_cf_drop_and_recreate(db.db, C.uint8_t(cf), &cErr.p)
	if err := cErr.Error(); err != nil {
		return fmt.Errorf("drop and recreate cf %d: %w", cf, err)
	}
	return nil
}
