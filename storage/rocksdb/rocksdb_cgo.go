//go:build cgo

package rocksdb

/*
#cgo LDFLAGS: -L./connector/rocksdb_vendor -lrocksdb
#cgo LDFLAGS: -lstdc++ -lm -lpthread -ldl -lsnappy -lz -lbz2 -llz4 -lzstd -luring
#cgo CFLAGS: -I./connector
#include "db.h"
#include "perf.h"
#include "read.h"
#include "stats.h"
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
	arenaChan      chan *Arena
	db             *C.db_t
	idMutexes      *helpers.KeyedMutex[uint32]
	cachePerCF     []bool
	metricsEnabled bool
	arenaSize      uint64
	arenaTTL       uint8
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
	var cachePerCF = make([]bool, len(opts.Rocks.ColumnFamilies)+1)
	for i := range cachePerCF {
		cachePerCF[i] = opts.Rocks.BlockCacheSize == nil
		if !cachePerCF[i] && i > 0 {
			var table = opts.Rocks.ColumnFamilies[i-1].Table
			cachePerCF[i] = table != nil && table.BlockCacheSize != nil
		}
	}
	return &DB{
		arenaChan:      arenaChan,
		db:             db,
		idMutexes:      helpers.NewKeyedMutex[uint32](),
		cachePerCF:     cachePerCF,
		metricsEnabled: opts.Rocks.StatisticsLevel != nil && *opts.Rocks.StatisticsLevel > 0,
		arenaSize:      opts.ArenaIntitialSize,
		arenaTTL:       opts.ArenaTTL,
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

func (db *DB) Stats() (*Stats, error) {
	var cDB C.db_stats_t
	var cErr CError
	C.db_stats(db.db, &cDB, &cErr.p)
	if err := cErr.Error(); err != nil {
		return nil, fmt.Errorf("db stats: %w", err)
	}
	var stats = &Stats{
		DB: DBStats{
			BackgroundErrors:          uint64(cDB.background_errors),
			NumRunningCompactions:     uint64(cDB.num_running_compactions),
			NumRunningFlushes:         uint64(cDB.num_running_flushes),
			NumUnscheduledCompactions: uint64(cDB.num_unscheduled_compactions),
			IsWriteStopped:            cDB.is_write_stopped != 0,
			NumSnapshots:              uint64(cDB.num_snapshots),
			OldestSnapshotTime:        uint64(cDB.oldest_snapshot_time),
		},
		ColumnFamilies: make([]CFStats, int(db.db.cf_count)),
	}
	for cf := range stats.ColumnFamilies {
		var cCF C.cf_stats_t
		C.db_cf_stats(db.db, C.uint8_t(cf), &cCF, &cErr.p)
		if err := cErr.Error(); err != nil {
			return nil, fmt.Errorf("cf %d stats: %w", cf, err)
		}
		stats.ColumnFamilies[cf] = CFStats{
			EstimateNumKeys:          uint64(cCF.estimate_num_keys),
			NumEntriesActiveMemTable: uint64(cCF.num_entries_active_mem_table),
			NumEntriesImmMemTables:   uint64(cCF.num_entries_imm_mem_tables),
			NumDeletesActiveMemTable: uint64(cCF.num_deletes_active_mem_table),
			NumDeletesImmMemTables:   uint64(cCF.num_deletes_imm_mem_tables),
			CurSizeActiveMemTable:    uint64(cCF.cur_size_active_mem_table),
			CurSizeAllMemTables:      uint64(cCF.cur_size_all_mem_tables),
			SizeAllMemTables:         uint64(cCF.size_all_mem_tables),
			NumImmutableMemTable:     uint64(cCF.num_immutable_mem_table),
			MemTableFlushPending:     cCF.mem_table_flush_pending != 0,
			CompactionPending:        cCF.compaction_pending != 0,
			TotalSSTFilesSize:        uint64(cCF.total_sst_files_size),
			LiveSSTFilesSize:         uint64(cCF.live_sst_files_size),
			ObsoleteSSTFilesSize:     uint64(cCF.obsolete_sst_files_size),
			NumFilesAtLevel: [7]uint64{
				uint64(cCF.num_files_at_level[0]),
				uint64(cCF.num_files_at_level[1]),
				uint64(cCF.num_files_at_level[2]),
				uint64(cCF.num_files_at_level[3]),
				uint64(cCF.num_files_at_level[4]),
				uint64(cCF.num_files_at_level[5]),
				uint64(cCF.num_files_at_level[6]),
			},
			EstimateTableReadersMem:        uint64(cCF.estimate_table_readers_mem),
			EstimatePendingCompactionBytes: uint64(cCF.estimate_pending_compaction_bytes),
			EstimateLiveDataSize:           uint64(cCF.estimate_live_data_size),
			BaseLevel:                      uint64(cCF.base_level),
			BlockCacheUsage:                uint64(cCF.block_cache_usage),
			BlockCachePinnedUsage:          uint64(cCF.block_cache_pinned_usage),
			BlockCacheCapacity:             uint64(cCF.block_cache_capacity),
			NumLiveVersions:                uint64(cCF.num_live_versions),
		}
	}
	var sharedCacheUsage uint64
	for i, cfStats := range stats.ColumnFamilies {
		stats.Memory.MemTableTotal += cfStats.SizeAllMemTables
		stats.Memory.MemTableUnflushed += cfStats.CurSizeAllMemTables
		stats.Memory.TableReaders += cfStats.EstimateTableReadersMem
		if db.cachePerCF[i] {
			stats.Memory.CacheTotal += cfStats.BlockCacheUsage
		} else {
			sharedCacheUsage = cfStats.BlockCacheUsage
		}
	}
	stats.Memory.CacheTotal += sharedCacheUsage
	return stats, nil
}

func (db *DB) Metrics() (*Metrics, error) {
	if !db.metricsEnabled {
		return nil, ErrMetricsDisabled
	}
	var cMetrics C.metrics_t
	var cErr CError
	C.db_metrics(db.db, &cMetrics, &cErr.p)
	if err := cErr.Error(); err != nil {
		return nil, fmt.Errorf("metrics: %w", err)
	}
	var metrics = &Metrics{}
	for i, count := range cMetrics.tickers {
		metrics.Tickers[i] = uint64(count)
	}
	for i, histogram := range cMetrics.histograms {
		metrics.Histograms[i] = HistogramData{
			Count:   uint64(histogram.count),
			Sum:     uint64(histogram.sum),
			Min:     float64(histogram.min),
			Max:     float64(histogram.max),
			Median:  float64(histogram.median),
			P95:     float64(histogram.p95),
			P99:     float64(histogram.p99),
			Average: float64(histogram.average),
			StdDev:  float64(histogram.std_dev),
		}
	}
	return metrics, nil
}

func (db *DB) WithPerf(level PerfLevel, fn func() error) (*PerfMetrics, error) {
	if fn == nil {
		return nil, fmt.Errorf("perf: fn must not be nil")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var cPerf C.perf_t
	var cErr CError
	C.perf_open(&cPerf, C.int(int(level)), &cErr.p)
	if err := cErr.Error(); err != nil {
		return nil, fmt.Errorf("perf: %w", err)
	}
	defer C.perf_close(&cPerf)
	var fnErr = fn()
	var cMetrics C.perf_metrics_t
	C.perf_metrics(&cPerf, &cMetrics, &cErr.p)
	if err := cErr.Error(); err != nil {
		return nil, fmt.Errorf("perf: %w", err)
	}
	var metrics = &PerfMetrics{}
	for i, value := range cMetrics.values {
		metrics.Values[i] = uint64(value)
	}
	return metrics, fnErr
}
