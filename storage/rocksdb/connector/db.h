// Copyright 2026 KrainovSD
#ifndef STORAGE_ROCKSDB_CONNECTOR_DB_H_
#define STORAGE_ROCKSDB_CONNECTOR_DB_H_
#include "cf.h"
#include "helpers.h"
#include "rocksdb_vendor/c.h"
#include <stddef.h>
#include <stdint.h>

typedef struct {
    opt_bool_t fill_cache;
    opt_bool_t verify_checksums;
    opt_bool_t total_order_seek;
    opt_bool_t prefix_same_as_start;
} db_read_opts_t;

typedef struct {
    opt_bool_t sync;
    opt_bool_t disable_wal;
    opt_bool_t no_slowdown;
    opt_bool_t low_pri;
} db_write_opts_t;

typedef struct {
    cf_opts_t **cf_opts;
    size_t cf_count;

    db_write_opts_t **writer_opts;
    size_t writer_count;
    db_read_opts_t **reader_opts;
    size_t reader_count;

    opt_int_t compaction_type;
    opt_u64_t memtable_memory_budget;
    opt_int_t max_background_jobs;
    opt_u64_t max_total_wal_size;
    opt_size_t write_buffer_size;
    opt_bool_t avoid_unnecessary_blocking_io;

    opt_size_t keep_log_file_num;
    opt_size_t max_log_file_size;
    opt_unsigned_t stats_dump_period_sec;
    opt_u64_t delete_obsolete_files_period_micros;

    opt_int_t compression;
    const int *compression_levels;
    size_t compression_levels_size;
    opt_int_t max_open_files;
} db_opts_t;

typedef struct {
    rocksdb_column_family_handle_t **cf_handles;
    const char **cf_names;
    rocksdb_options_t **cf_options;
    size_t cf_count;
    rocksdb_readoptions_t **read_options;
    size_t reader_count;
    rocksdb_writeoptions_t **write_options;
    size_t writer_count;
    rocksdb_t *instance;
} db_t;

db_t *db_open(const char *path, db_opts_t *cfg, char **err);

void db_close(db_t *db);

void db_cf_drop_and_recreate(db_t *db, uint8_t cf_index, char **err);

void db_is_valid(db_t *db, char **err);

void db_reader_is_valid(db_t *db, uint8_t reader_index, char **err);

void db_writer_is_valid(db_t *db, uint8_t writer_index, char **err);

void db_cf_is_valid(db_t *db, uint8_t cf_index, char **err);

#endif // STORAGE_ROCKSDB_CONNECTOR_DB_H_
