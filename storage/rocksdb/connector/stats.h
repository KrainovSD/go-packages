// Copyright 2026 KrainovSD
#ifndef STORAGE_ROCKSDB_CONNECTOR_STATS_H_
#define STORAGE_ROCKSDB_CONNECTOR_STATS_H_
#include "db.h"
#include "rocksdb_vendor/c.h"
#include <stddef.h>
#include <stdint.h>

#define CF_STATS_LEVELS 7

typedef struct {
    uint64_t estimate_num_keys;
    uint64_t num_entries_active_mem_table;
    uint64_t num_entries_imm_mem_tables;
    uint64_t num_deletes_active_mem_table;
    uint64_t num_deletes_imm_mem_tables;
    uint64_t cur_size_active_mem_table;
    uint64_t cur_size_all_mem_tables;
    uint64_t size_all_mem_tables;
    uint64_t num_immutable_mem_table;
    uint64_t mem_table_flush_pending;
    uint64_t compaction_pending;
    uint64_t total_sst_files_size;
    uint64_t live_sst_files_size;
    uint64_t obsolete_sst_files_size;
    uint64_t num_files_at_level[CF_STATS_LEVELS];
    uint64_t estimate_table_readers_mem;
    uint64_t estimate_pending_compaction_bytes;
    uint64_t estimate_live_data_size;
    uint64_t base_level;
    uint64_t block_cache_usage;
    uint64_t block_cache_pinned_usage;
    uint64_t block_cache_capacity;
    uint64_t num_live_versions;
} cf_stats_t;

typedef struct {
    uint64_t background_errors;
    uint64_t num_running_compactions;
    uint64_t num_running_flushes;
    uint64_t num_unscheduled_compactions;
    uint64_t is_write_stopped;
    uint64_t num_snapshots;
    uint64_t oldest_snapshot_time;
} db_stats_t;

#define METRICS_TICKERS_COUNT 267
#define METRICS_HISTOGRAMS_COUNT 80

typedef struct {
    uint64_t count;
    uint64_t sum;
    double min;
    double max;
    double median;
    double p95;
    double p99;
    double average;
    double std_dev;
} metrics_histogram_t;

typedef struct {
    uint64_t tickers[METRICS_TICKERS_COUNT];
    metrics_histogram_t histograms[METRICS_HISTOGRAMS_COUNT];
} metrics_t;

void db_stats(db_t *db, db_stats_t *out, char **err);

void db_cf_stats(db_t *db, uint8_t cf_index, cf_stats_t *out, char **err);

void db_metrics(db_t *db, metrics_t *out, char **err);

#endif // STORAGE_ROCKSDB_CONNECTOR_STATS_H_
