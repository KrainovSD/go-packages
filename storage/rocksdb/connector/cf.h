// Copyright 2026 KrainovSD
#ifndef STORAGE_ROCKSDB_CONNECTOR_CF_H_
#define STORAGE_ROCKSDB_CONNECTOR_CF_H_
#include "helpers.h"
#include "rocksdb_vendor/c.h"
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

typedef struct {
    opt_double_t bloom_bits_per_key;
    opt_bool_t bloom_whole_key_filering;
    opt_size_t block_cache_size;
} cf_table_opts_t;

typedef struct {
    const char *name;

    size_t prefix_len;
    size_t prefix_offset;
    uint8_t comparator_type;
    rocksdb_mergeoperator_t *merge_operator;

    opt_int_t compaction_type;
    opt_u64_t memtable_memory_budget;
    opt_int_t max_background_jobs;

    opt_size_t write_buffer_size;
    opt_int_t min_write_buffer_number_to_merge;
    opt_int_t max_write_buffer_number;

    opt_u64_t target_file_size_base;
    opt_int_t target_file_size_multiplier;
    opt_u64_t max_bytes_for_level_base;
    opt_double_t max_bytes_for_level_multiplier;

    opt_bool_t level_compaction_dynamic_level_bytes;
    opt_int_t level0_file_num_compaction_trigger;
    opt_int_t level0_slowdown_writes_trigger;
    opt_int_t level0_stop_writes_trigger;

    opt_int_t compression;
    const int *compression_levels;
    size_t compression_levels_size;
    opt_int_t max_open_files;
    opt_u64_t ttl;

    cf_table_opts_t *table_opts;
} cf_opts_t;

enum cf_comparator_type { CF_COMPARATOR_U64_LE = 1 };

rocksdb_options_t *cf_create(cf_opts_t *cfg);

#endif // STORAGE_ROCKSDB_CONNECTOR_CF_H_
