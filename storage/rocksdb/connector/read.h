// Copyright 2026 KrainovSD
#ifndef STORAGE_ROCKSDB_CONNECTOR_READ_H_
#define STORAGE_ROCKSDB_CONNECTOR_READ_H_
#include "arena.h"
#include "db.h"
#include "rocksdb_vendor/c.h"

typedef struct {
    char *prefix;
    uint64_t prefix_len;
    uint8_t cf_index;
    uint8_t reader_index;
    uint64_t limit;
    uint64_t offset;
    char *cursor;
    size_t cursor_len;
    bool reverse;
    bool keys;
} db_read_all_opts;

void db_read(db_t *db, uint8_t cf_index, uint8_t reader_index, const char *key,
             size_t key_len, size_t *value_len, db_arena_t *arena, char **err);

void db_read_all(db_t *db, db_read_all_opts *opts, db_arena_t *arena,
                 char **err);

void db_read_batch_single_cf(db_t *db, uint8_t cf_index, uint8_t reader_index,
                             const rocksdb_slice_t *keys, size_t keys_count,
                             bool sorted_keys, db_arena_t *arena, char **err);

void db_read_batch_multi_cf(db_t *db, uint8_t *cf_indexes, uint8_t reader_index,
                            const char **keys, size_t *key_lens,
                            size_t keys_count, db_arena_t *arena, char **err);

#endif // STORAGE_ROCKSDB_CONNECTOR_READ_H_
