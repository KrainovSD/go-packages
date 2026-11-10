// Copyright 2026 KrainovSD
#include "read.h"
#include "arena.h"
#include "db.h"
#include "rocksdb_vendor/c.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

void db_read(db_t *db, uint8_t cf_index, uint8_t reader_index, const char *key,
             size_t key_len, size_t *value_len, db_arena_t *arena, char **err) {
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
    db_arena_is_valid(arena, err);
    if (*err) {
        return;
    }
    db_arena_reset(arena, err);
    if (*err) {
        return;
    }
    rocksdb_pinnable_handle_t *h =
        rocksdb_get_pinned_cf_v2(db->instance, db->read_options[reader_index],
                                 db->cf_handles[cf_index], key, key_len, err);
    if (*err) {
        return;
    }
    if (!h) {
        return;
    }
    const char *inner = rocksdb_pinnable_handle_get_value(h, value_len);
    if (*value_len == 0) {
        rocksdb_pinnable_handle_destroy(h);
        return;
    }
    db_arena_append(arena, (char *)inner, *value_len, err);
    rocksdb_pinnable_handle_destroy(h);
    return;
}

void db_read_all(db_t *db, db_read_all_opts *opts, db_arena_t *arena,
                 char **err) {
    db_is_valid(db, err);
    if (*err) {
        return;
    }
    db_cf_is_valid(db, opts->cf_index, err);
    if (*err) {
        return;
    }
    db_reader_is_valid(db, opts->reader_index, err);
    if (*err) {
        return;
    }
    db_arena_is_valid(arena, err);
    if (*err) {
        return;
    }
    db_arena_reset(arena, err);
    if (*err) {
        return;
    }
    rocksdb_readoptions_t *reader = db->read_options[opts->reader_index];
    rocksdb_column_family_handle_t *cf_handle = db->cf_handles[opts->cf_index];
    uint8_t prefix = rocksdb_readoptions_get_prefix_same_as_start(reader);
    rocksdb_iterator_t *iter =
        rocksdb_create_iterator_cf(db->instance, reader, cf_handle);
    if (!iter) {
        *err = strdup("create iterator");
        return;
    }
    if (prefix && opts->prefix != NULL) {
        rocksdb_iter_seek(iter, opts->prefix, opts->prefix_len);
    } else if (!opts->reverse) {
        rocksdb_iter_seek_to_first(iter);
    } else {
        rocksdb_iter_seek_to_last(iter);
    }
    uint64_t offset = opts->offset;
    char *cursor = opts->cursor;
    size_t cursor_len = opts->cursor_len;
    bool keys = opts->keys;
    uint64_t limit = opts->limit;
    while (rocksdb_iter_valid(iter)) {
        if (cursor != NULL) {
            size_t key_len;
            const char *key = rocksdb_iter_key(iter, &key_len);
            if (key_len == cursor_len && memcmp(key, cursor, key_len) == 0) {
                cursor = NULL;
            }
            goto NEXT_ITERATION;
        }
        if (offset > 0) {
            offset--;
            goto NEXT_ITERATION;
        }
        if (keys) {
            size_t key_len;
            const char *key = rocksdb_iter_key(iter, &key_len);
            db_arena_append(arena, (char *)key, key_len, err);
        }
        size_t value_len;
        const char *value = rocksdb_iter_value(iter, &value_len);
        db_arena_append(arena, (char *)value, value_len, err);
        if (*err) {
            rocksdb_iter_destroy(iter);
            return;
        }
        limit--;
        if (limit == 0) {
            break;
        }

    NEXT_ITERATION:
        if (!opts->reverse) {
            rocksdb_iter_next(iter);
        } else {
            rocksdb_iter_prev(iter);
        }
    }
    rocksdb_iter_get_error(iter, err);
    rocksdb_iter_destroy(iter);
}

void db_read_batch_single_cf(db_t *db, uint8_t cf_index, uint8_t reader_index,
                             const rocksdb_slice_t *keys, size_t keys_count,
                             bool sorted_keys, db_arena_t *arena, char **err) {
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
    db_arena_is_valid(arena, err);
    if (*err) {
        return;
    }
    db_arena_reset(arena, err);
    if (*err) {
        return;
    }
    rocksdb_pinnable_multi_get_t *mg = rocksdb_batched_multi_get_pinned_cf(
        db->instance, db->read_options[reader_index], db->cf_handles[cf_index],
        keys_count, keys, sorted_keys);
    for (size_t i = 0; i < rocksdb_pinnable_multi_get_count(mg); i++) {
        const char *value;
        size_t value_len;
        const char *error;
        size_t error_len;

        int res = rocksdb_pinnable_multi_get_result(mg, i, &value, &value_len,
                                                    &error, &error_len);
        if (res == rocksdb_pinnable_multi_get_error) {
            *err = malloc(error_len + 1);
            if (*err != NULL) {
                memcpy(*err, error, error_len);
                (*err)[error_len] = '\0';
            }
            goto cleanup;
        }
        if (res == rocksdb_pinnable_multi_get_not_found) {
            value_len = 0;
            value = NULL;
        }
        db_arena_append(arena, (char *)value, value_len, err);
        if (*err) {
            goto cleanup;
        }
    }
cleanup:
    if (mg) {
        rocksdb_pinnable_multi_get_destroy(mg);
    }
}

void db_read_batch_multi_cf(db_t *db, uint8_t *cf_indexes, uint8_t reader_index,
                            const char **keys, size_t *key_lens,
                            size_t keys_count, db_arena_t *arena, char **err) {
    db_is_valid(db, err);
    if (*err) {
        return;
    }
    db_reader_is_valid(db, reader_index, err);
    if (*err) {
        return;
    }
    db_arena_is_valid(arena, err);
    if (*err) {
        return;
    }
    db_arena_reset(arena, err);
    if (*err) {
        return;
    }
    char **values = (char **)malloc(keys_count * sizeof(*values));
    size_t *value_lens = (size_t *)malloc(keys_count * sizeof(*value_lens));
    char **errs = (char **)malloc(keys_count * sizeof(*values));
    const rocksdb_column_family_handle_t **cf_handles =
        (const rocksdb_column_family_handle_t **)malloc(keys_count *
                                                        sizeof(*cf_handles));
    if (!values || !value_lens || !errs || !cf_handles) {
        *err = strdup("oom");
        goto cleanup;
    }
    for (size_t i = 0; i < keys_count; i++) {
        values[i] = NULL;
        value_lens[i] = 0;
        errs[i] = NULL;
        db_cf_is_valid(db, cf_indexes[i], err);
        if (*err) {
            goto cleanup;
        }
        cf_handles[i] = db->cf_handles[cf_indexes[i]];
    }
    rocksdb_multi_get_cf(db->instance, db->read_options[reader_index],
                         cf_handles, keys_count, keys, key_lens, values,
                         value_lens, errs);
    for (size_t i = 0; i < keys_count; i++) {
        if (errs[i] != NULL) {
            *err = strdup(errs[i]);
            goto cleanup;
        }
        size_t value_len = (size_t)value_lens[i];
        char *value = values[i];
        if (value == NULL) {
            value_len = 0;
        }
        db_arena_append(arena, value, value_len, err);
        if (*err) {
            goto cleanup;
        }
    }
cleanup:
    if (values) {
        for (size_t i = 0; i < keys_count; i++) {
            if (values[i] != NULL) {
                free(values[i]);
                values[i] = NULL;
            }
        }
        free(values);
    }
    if (value_lens) {
        free(value_lens);
    }
    if (cf_handles) {
        free(cf_handles);
    }
    if (errs) {
        size_t total_len = 0;
        for (size_t i = 0; i < keys_count; i++) {
            if (errs[i] != NULL) {
                total_len += strlen(errs[i]) + 1;
            }
        }
        if (total_len > 0) {
            *err = malloc(total_len + 1);
        }
        size_t offset = 0;
        for (size_t i = 0; i < keys_count; i++) {
            if (errs[i] != NULL) {
                if (*err != NULL) {
                    int written = snprintf(
                        *err + offset, total_len + 1 - offset, "%s\n", errs[i]);
                    if (written > 0) {
                        offset += (size_t)written;
                    }
                }
                free(errs[i]);
                errs[i] = NULL;
            }
        }
        free(errs);
    }
}
