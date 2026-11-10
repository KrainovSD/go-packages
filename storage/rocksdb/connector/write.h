// Copyright 2026 KrainovSD
#ifndef STORAGE_ROCKSDB_CONNECTOR_WRITE_H_
#define STORAGE_ROCKSDB_CONNECTOR_WRITE_H_
#include "db.h"
#include "rocksdb_vendor/c.h"

enum db_write_oprations {
    DB_OPERATION_PUT = 1,
    DB_OPERATION_MERGE = 2,
    DB_OPERATION_DELETE = 3,
};

typedef struct {
    const char *key;
    size_t key_len;
    const char *value;
    size_t value_len;
    uint8_t cf_index;
    uint8_t *reader_index;
    uint8_t operation;
} db_write_batch_opts_t;

void db_write(db_t *db, uint8_t cf_index, uint8_t writer_index,
              uint8_t operation, const char *key, size_t key_len,
              const char *value, size_t value_len, char **err);

void db_write_batch(db_t *db, uint8_t writer_index,
                    const db_write_batch_opts_t **data, size_t data_count,
                    char **err);

void db_auto_increment(db_t *db, uint8_t cf_index, uint8_t reader_index,
                       uint8_t writer_index, const char *key, size_t key_len,
                       uint64_t *next_id, char **err);

#endif // STORAGE_ROCKSDB_CONNECTOR_WRITE_H_
