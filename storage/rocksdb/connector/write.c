// Copyright 2026 KrainovSD
#include "write.h"
#include "db.h"
#include "rocksdb_vendor/c.h"
#include <endian.h>
#include <stdlib.h>
#include <string.h>

void db_write(db_t *db, uint8_t cf_index, uint8_t writer_index,
              uint8_t operation, const char *key, size_t key_len,
              const char *value, size_t value_len, char **err) {
    db_is_valid(db, err);
    if (*err) {
        return;
    }
    db_cf_is_valid(db, cf_index, err);
    if (*err) {
        return;
    }
    db_writer_is_valid(db, writer_index, err);
    if (*err) {
        return;
    }
    switch (operation) {
    case DB_OPERATION_PUT:
        rocksdb_put_cf(db->instance, db->write_options[writer_index],
                       db->cf_handles[cf_index], key, key_len, value, value_len,
                       err);
        break;
    case DB_OPERATION_MERGE:
        rocksdb_merge_cf(db->instance, db->write_options[writer_index],
                         db->cf_handles[cf_index], key, key_len, value,
                         value_len, err);
        break;
    case DB_OPERATION_DELETE:
        rocksdb_delete_cf(db->instance, db->write_options[writer_index],
                          db->cf_handles[cf_index], key, key_len, err);
        break;
    default:
        break;
    }
}

void db_write_batch(db_t *db, uint8_t writer_index,
                    const db_write_batch_opts_t **data, size_t data_count,
                    char **err) {
    db_is_valid(db, err);
    if (*err) {
        return;
    }
    db_writer_is_valid(db, writer_index, err);
    if (*err) {
        return;
    }
    rocksdb_writebatch_t *batch = rocksdb_writebatch_create();
    rocksdb_writeoptions_t *writer = db->write_options[writer_index];
    for (size_t i = 0; i < data_count; i++) {
        const db_write_batch_opts_t *opts = data[i];
        db_cf_is_valid(db, opts->cf_index, err);
        if (*err) {
            break;
        }
        if (opts->reader_index != NULL) {
            uint8_t reader_index = *opts->reader_index;
            db_reader_is_valid(db, reader_index, err);
            if (*err) {
                break;
            }
            rocksdb_readoptions_t *reader = db->read_options[reader_index];
            uint64_t prefix =
                rocksdb_readoptions_get_prefix_same_as_start(reader);
            if (prefix) {
                rocksdb_iterator_t *iter = rocksdb_create_iterator_cf(
                    db->instance, reader, db->cf_handles[opts->cf_index]);
                if (iter == NULL) {
                    *err = strdup("create iter");
                    break;
                }
                rocksdb_iter_seek(iter, opts->key, opts->key_len);
                while (rocksdb_iter_valid(iter)) {
                    const char *key;
                    size_t key_len;
                    key = rocksdb_iter_key(iter, &key_len);
                    if (key && key_len > 0) {
                        switch (opts->operation) {
                        case DB_OPERATION_PUT:
                            rocksdb_writebatch_put_cf(
                                batch, db->cf_handles[opts->cf_index], key,
                                key_len, opts->value, opts->value_len);
                            break;
                        case DB_OPERATION_MERGE:
                            rocksdb_writebatch_merge_cf(
                                batch, db->cf_handles[opts->cf_index], key,
                                key_len, opts->value, opts->value_len);
                            break;
                        case DB_OPERATION_DELETE:
                            rocksdb_writebatch_delete_cf(
                                batch, db->cf_handles[opts->cf_index], key,
                                key_len);
                            break;
                        default:
                            break;
                        }
                    }
                    rocksdb_iter_next(iter);
                }
                rocksdb_iter_get_error(iter, err);
                rocksdb_iter_destroy(iter);
                if (*err) {
                    break;
                }
                continue;
            }
        }
        switch (opts->operation) {
        case DB_OPERATION_PUT:
            rocksdb_writebatch_put_cf(batch, db->cf_handles[opts->cf_index],
                                      opts->key, opts->key_len, opts->value,
                                      opts->value_len);
            break;
        case DB_OPERATION_MERGE:
            rocksdb_writebatch_merge_cf(batch, db->cf_handles[opts->cf_index],
                                        opts->key, opts->key_len, opts->value,
                                        opts->value_len);
            break;
        case DB_OPERATION_DELETE:
            rocksdb_writebatch_delete_cf(batch, db->cf_handles[opts->cf_index],
                                         opts->key, opts->key_len);
            break;
        default:
            break;
        }
    }
    if (!*err) {
        rocksdb_write(db->instance, writer, batch, err);
    }
    rocksdb_writebatch_destroy(batch);
}

void db_auto_increment(db_t *db, uint8_t cf_index, uint8_t reader_index,
                       uint8_t writer_index, const char *key, size_t key_len,
                       uint64_t *next_id, char **err) {
    db_is_valid(db, err);
    if (*err) {
        return;
    }
    db_cf_is_valid(db, cf_index, err);
    if (*err) {
        return;
    }
    db_reader_is_valid(db, reader_index, err);
    if (*err) {
        return;
    }
    db_writer_is_valid(db, writer_index, err);
    if (*err) {
        return;
    }
    size_t value_len = 0;
    char *value =
        rocksdb_get_cf(db->instance, db->read_options[reader_index],
                       db->cf_handles[cf_index], key, key_len, &value_len, err);
    if (*err) {
        return;
    }
    uint64_t current = 0;
    if (value != NULL && value_len >= sizeof(uint64_t)) {
        current = le64toh(*(uint64_t *)value);
    }
    free(value);
    uint64_t next = current + 1;
    char buf[sizeof(uint64_t)];
    uint64_t le = htole64(next);
    memcpy(buf, &le, sizeof(buf));
    rocksdb_put_cf(db->instance, db->write_options[writer_index],
                   db->cf_handles[cf_index], key, key_len, buf, sizeof(buf),
                   err);
    if (*err) {
        return;
    }
    *next_id = next;
}
