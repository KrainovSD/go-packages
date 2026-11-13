//go:build cgo

package rocksdb

/*
#cgo CFLAGS: -I./connector
#include "db.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"
import "unsafe"

func (o *RocksDBOptions) toC(arena *Arena) *C.db_opts_t {
	if o == nil {
		return nil
	}
	var cLevels, cLevelsSize = compressionLevelsToC(arena, o.CompressionLevels)
	var cOpts = (*C.db_opts_t)(arena.alloc(unsafe.Sizeof(C.db_opts_t{}), unsafe.Alignof(C.db_opts_t{})))
	*cOpts = C.db_opts_t{
		cf_opts:                             cfOptsToC(arena, o.ColumnFamilies),
		cf_count:                            C.size_t(len(o.ColumnFamilies)),
		writer_opts:                         writerOptsToC(arena, o.Writers),
		writer_count:                        C.size_t(len(o.Writers)),
		reader_opts:                         readerOptsToC(arena, o.Readers),
		reader_count:                        C.size_t(len(o.Readers)),
		compaction_type:                     optInt(o.CompactionType),
		memtable_memory_budget:              optU64(o.MemtableMemoryBudget),
		max_background_jobs:                 optInt(o.MaxBackgroundJobs),
		max_total_wal_size:                  optU64(o.MaxTotalWalSize),
		write_buffer_size:                   optSize(o.WriteBufferSize),
		avoid_unnecessary_blocking_io:       optBool(o.AvoidUnnecessaryBlockingIO),
		keep_log_file_num:                   optSize(o.KeepLogFileNum),
		max_log_file_size:                   optSize(o.MaxLogFileSize),
		stats_dump_period_sec:               optUnsigned(o.StatsDumpPeriodSec),
		delete_obsolete_files_period_micros: optU64(o.DeleteObsoleteFilesPeriodMicros),
		compression:                         optInt(o.Compression),
		compression_levels:                  cLevels,
		compression_levels_size:             cLevelsSize,
		max_open_files:                      optInt(o.MaxOpenFiles),
		block_cache_size:                    optSize(o.BlockCacheSize),
		statistics_level:                    optInt(o.StatisticsLevel),
	}
	return cOpts
}

func (o *ColumnFamilyOptions) toC(arena *Arena) *C.cf_opts_t {
	var cLevels, cLevelsSize = compressionLevelsToC(arena, o.CompressionLevels)
	var cOpts = (*C.cf_opts_t)(arena.alloc(unsafe.Sizeof(C.cf_opts_t{}), unsafe.Alignof(C.cf_opts_t{})))
	*cOpts = C.cf_opts_t{
		name:                                 arena.cString(o.Name),
		prefix_len:                           C.size_t(o.PrefixLen),
		prefix_offset:                        C.size_t(o.PrefixOffset),
		comparator_type:                      C.uint8_t(o.ComparatorType),
		merge_operator:                       (*C.rocksdb_mergeoperator_t)(o.MergeOperator),
		compaction_type:                      optInt(o.CompactionType),
		memtable_memory_budget:               optU64(o.MemtableMemoryBudget),
		max_background_jobs:                  optInt(o.MaxBackgroundJobs),
		write_buffer_size:                    optSize(o.WriteBufferSize),
		min_write_buffer_number_to_merge:     optInt(o.MinWriteBufferNumberToMerge),
		max_write_buffer_number:              optInt(o.MaxWriteBufferNumber),
		target_file_size_base:                optU64(o.TargetFileSizeBase),
		target_file_size_multiplier:          optInt(o.TargetFileSizeMultiplier),
		max_bytes_for_level_base:             optU64(o.MaxBytesForLevelBase),
		max_bytes_for_level_multiplier:       optDouble(o.MaxBytesForLevelMultiplier),
		level_compaction_dynamic_level_bytes: optBool(o.LevelCompactionDynamicLevelBytes),
		level0_file_num_compaction_trigger:   optInt(o.Level0FileNumCompactionTrigger),
		level0_slowdown_writes_trigger:       optInt(o.Level0SlowdownWritesTrigger),
		level0_stop_writes_trigger:           optInt(o.Level0StopWritesTrigger),
		compression:                          optInt(o.Compression),
		compression_levels:                   cLevels,
		compression_levels_size:              cLevelsSize,
		max_open_files:                       optInt(o.MaxOpenFiles),
		ttl:                                  optU64(o.TTL),
		table_opts:                           o.Table.toC(arena),
	}
	return cOpts
}

func (o *TableOptions) toC(arena *Arena) *C.cf_table_opts_t {
	if o == nil {
		return nil
	}
	var cOpts = (*C.cf_table_opts_t)(arena.alloc(unsafe.Sizeof(C.cf_table_opts_t{}), unsafe.Alignof(C.cf_table_opts_t{})))
	*cOpts = C.cf_table_opts_t{
		bloom_bits_per_key:       optDouble(o.BloomBitsPerKey),
		bloom_whole_key_filering: optBool(o.BloomWholeKeyFiltering),
		block_cache_size:         optSize(o.BlockCacheSize),
	}
	return cOpts
}

func (o *MutableDBOptions) toC(arena *Arena) *C.db_opts_t {
	var cOpts = (*C.db_opts_t)(arena.alloc(unsafe.Sizeof(C.db_opts_t{}), unsafe.Alignof(C.db_opts_t{})))
	*cOpts = C.db_opts_t{
		max_background_jobs:                 optInt(o.MaxBackgroundJobs),
		max_total_wal_size:                  optU64(o.MaxTotalWalSize),
		stats_dump_period_sec:               optUnsigned(o.StatsDumpPeriodSec),
		delete_obsolete_files_period_micros: optU64(o.DeleteObsoleteFilesPeriodMicros),
		max_open_files:                      optInt(o.MaxOpenFiles),
	}
	return cOpts
}

func (o *MutableCFOptions) toC(arena *Arena) *C.cf_opts_t {
	var cLevels, cLevelsSize = compressionLevelsToC(arena, o.CompressionLevels)
	var cOpts = (*C.cf_opts_t)(arena.alloc(unsafe.Sizeof(C.cf_opts_t{}), unsafe.Alignof(C.cf_opts_t{})))
	*cOpts = C.cf_opts_t{
		write_buffer_size:                   optSize(o.WriteBufferSize),
		max_write_buffer_number:             optInt(o.MaxWriteBufferNumber),
		target_file_size_base:               optU64(o.TargetFileSizeBase),
		target_file_size_multiplier:         optInt(o.TargetFileSizeMultiplier),
		max_bytes_for_level_base:            optU64(o.MaxBytesForLevelBase),
		max_bytes_for_level_multiplier:      optDouble(o.MaxBytesForLevelMultiplier),
		level0_file_num_compaction_trigger:  optInt(o.Level0FileNumCompactionTrigger),
		level0_slowdown_writes_trigger:      optInt(o.Level0SlowdownWritesTrigger),
		level0_stop_writes_trigger:          optInt(o.Level0StopWritesTrigger),
		compression:                         optInt(o.Compression),
		compression_levels:                  cLevels,
		compression_levels_size:             cLevelsSize,
		ttl:                                 optU64(o.TTL),
	}
	return cOpts
}

func (o *WriterOptions) toC(arena *Arena) *C.db_write_opts_t {
	var cOpts = (*C.db_write_opts_t)(arena.alloc(unsafe.Sizeof(C.db_write_opts_t{}), unsafe.Alignof(C.db_write_opts_t{})))
	*cOpts = C.db_write_opts_t{
		sync:        optBool(o.Sync),
		disable_wal: optBool(o.DisableWal),
		no_slowdown: optBool(o.NoSlowdown),
		low_pri:     optBool(o.LowPri),
	}
	return cOpts
}

func (o *ReaderOptions) toC(arena *Arena) *C.db_read_opts_t {
	var cOpts = (*C.db_read_opts_t)(arena.alloc(unsafe.Sizeof(C.db_read_opts_t{}), unsafe.Alignof(C.db_read_opts_t{})))
	*cOpts = C.db_read_opts_t{
		fill_cache:           optBool(o.FillCache),
		verify_checksums:     optBool(o.VerifyChecksums),
		total_order_seek:     optBool(o.TotalOrderSeek),
		prefix_same_as_start: optBool(o.PrefixSameAsStart),
	}
	return cOpts
}

func cfOptsToC(arena *Arena, cfs []ColumnFamilyOptions) **C.cf_opts_t {
	if len(cfs) == 0 {
		return nil
	}
	var ptr = (**C.cf_opts_t)(arena.alloc(
		uintptr(len(cfs))*unsafe.Sizeof((*C.cf_opts_t)(nil)),
		unsafe.Alignof((*C.cf_opts_t)(nil)),
	))
	var items = unsafe.Slice(ptr, len(cfs))
	for i := range cfs {
		items[i] = cfs[i].toC(arena)
	}
	return ptr
}

func writerOptsToC(arena *Arena, writers []WriterOptions) **C.db_write_opts_t {
	if len(writers) == 0 {
		return nil
	}
	var ptr = (**C.db_write_opts_t)(arena.alloc(
		uintptr(len(writers))*unsafe.Sizeof((*C.db_write_opts_t)(nil)),
		unsafe.Alignof((*C.db_write_opts_t)(nil)),
	))
	var items = unsafe.Slice(ptr, len(writers))
	for i := range writers {
		items[i] = writers[i].toC(arena)
	}
	return ptr
}

func readerOptsToC(arena *Arena, readers []ReaderOptions) **C.db_read_opts_t {
	if len(readers) == 0 {
		return nil
	}
	var ptr = (**C.db_read_opts_t)(arena.alloc(
		uintptr(len(readers))*unsafe.Sizeof((*C.db_read_opts_t)(nil)),
		unsafe.Alignof((*C.db_read_opts_t)(nil)),
	))
	var items = unsafe.Slice(ptr, len(readers))
	for i := range readers {
		items[i] = readers[i].toC(arena)
	}
	return ptr
}

func compressionLevelsToC(arena *Arena, levels []Compression) (*C.int, C.size_t) {
	if len(levels) == 0 {
		return nil, 0
	}
	var ptr = (*C.int)(arena.alloc(
		uintptr(len(levels))*unsafe.Sizeof(C.int(0)),
		unsafe.Alignof(C.int(0)),
	))
	var items = unsafe.Slice(ptr, len(levels))
	for i, level := range levels {
		items[i] = C.int(level)
	}
	return ptr, C.size_t(len(levels))
}

func optInt[T ~int](v *T) C.opt_int_t {
	if v == nil {
		return C.opt_int_t{}
	}
	return C.opt_int_t{has: true, v: C.int(*v)}
}

func optSize(v *uint64) C.opt_size_t {
	if v == nil {
		return C.opt_size_t{}
	}
	return C.opt_size_t{has: true, v: C.size_t(*v)}
}

func optU64(v *uint64) C.opt_u64_t {
	if v == nil {
		return C.opt_u64_t{}
	}
	return C.opt_u64_t{has: true, v: C.uint64_t(*v)}
}

func optBool(v *bool) C.opt_bool_t {
	if v == nil {
		return C.opt_bool_t{}
	}
	return C.opt_bool_t{has: true, v: C.bool(*v)}
}

func optUnsigned(v *uint32) C.opt_unsigned_t {
	if v == nil {
		return C.opt_unsigned_t{}
	}
	return C.opt_unsigned_t{has: true, v: C.uint(*v)}
}

func optDouble(v *float64) C.opt_double_t {
	if v == nil {
		return C.opt_double_t{}
	}
	return C.opt_double_t{has: true, v: C.double(*v)}
}
