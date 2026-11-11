// Copyright 2026 KrainovSD
#include "db.h"
#include "cf.h"
#include "rocksdb_vendor/c.h"
#include <assert.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static void db_apply_opts(rocksdb_options_t *opts, db_opts_t *cfg) {
    if (cfg == NULL) {
        return;
    }
    if (cfg->compaction_type.has) {
        uint64_t memory_budget = 0;
        if (cfg->memtable_memory_budget.has) {
            memory_budget = cfg->memtable_memory_budget.v;
        }
        switch (cfg->compaction_type.v) {
        case rocksdb_level_compaction:
            rocksdb_options_optimize_level_style_compaction(opts,
                                                            memory_budget);
            break;
        case rocksdb_universal_compaction:
            rocksdb_options_optimize_universal_style_compaction(opts,
                                                                memory_budget);
            break;
        }
    }
    if (cfg->max_background_jobs.has) {
        rocksdb_options_set_max_background_jobs(opts,
                                                cfg->max_background_jobs.v);
    } else {
        int64_t cpus = sysconf(_SC_NPROCESSORS_ONLN);
        rocksdb_options_set_max_background_jobs(opts, cpus);
    }
    if (cfg->max_total_wal_size.has) {
        rocksdb_options_set_max_total_wal_size(opts, cfg->max_total_wal_size.v);
    }
    if (cfg->write_buffer_size.has) {
        rocksdb_options_set_write_buffer_size(opts, cfg->write_buffer_size.v);
    }
    if (cfg->avoid_unnecessary_blocking_io.has) {
        rocksdb_options_set_avoid_unnecessary_blocking_io(
            opts, cfg->avoid_unnecessary_blocking_io.v ? 1 : 0);
    }

    if (cfg->keep_log_file_num.has) {
        rocksdb_options_set_keep_log_file_num(opts, cfg->keep_log_file_num.v);
    }
    if (cfg->max_log_file_size.has) {
        rocksdb_options_set_max_log_file_size(opts, cfg->max_log_file_size.v);
    }
    if (cfg->stats_dump_period_sec.has) {
        rocksdb_options_set_stats_dump_period_sec(opts,
                                                  cfg->stats_dump_period_sec.v);
    }
    if (cfg->delete_obsolete_files_period_micros.has) {
        rocksdb_options_set_delete_obsolete_files_period_micros(
            opts, cfg->delete_obsolete_files_period_micros.v);
    }

    if (cfg->compression_levels != NULL && cfg->compression_levels_size > 0) {
        rocksdb_options_set_compression_per_level(opts, cfg->compression_levels,
                                                  cfg->compression_levels_size);
    } else if (cfg->compression.has) {
        rocksdb_options_set_compression(opts, cfg->compression.v);
    }

    if (cfg->max_open_files.has) {
        rocksdb_options_set_max_open_files(opts, cfg->max_open_files.v);
    }
}

static rocksdb_readoptions_t *db_create_reader(db_read_opts_t *opts) {
    rocksdb_readoptions_t *reader = rocksdb_readoptions_create();
    if (opts->fill_cache.has) {
        rocksdb_readoptions_set_fill_cache(reader, opts->fill_cache.v ? 1 : 0);
    }
    if (opts->verify_checksums.has) {
        rocksdb_readoptions_set_verify_checksums(
            reader, opts->verify_checksums.v ? 1 : 0);
    }
    if (opts->total_order_seek.has) {
        rocksdb_readoptions_set_total_order_seek(
            reader, opts->total_order_seek.v ? 1 : 0);
    }
    if (opts->prefix_same_as_start.has) {
        rocksdb_readoptions_set_prefix_same_as_start(
            reader, opts->prefix_same_as_start.v ? 1 : 0);
    }
    return reader;
}

static rocksdb_writeoptions_t *db_create_writer(db_write_opts_t *opts) {
    rocksdb_writeoptions_t *writer = rocksdb_writeoptions_create();
    if (opts->sync.has) {
        rocksdb_writeoptions_set_sync(writer, opts->sync.v ? 1 : 0);
    }
    if (opts->disable_wal.has) {
        rocksdb_writeoptions_disable_WAL(writer, opts->disable_wal.v ? 1 : 0);
    }
    if (opts->no_slowdown.has) {
        rocksdb_writeoptions_set_no_slowdown(writer,
                                             opts->no_slowdown.v ? 1 : 0);
    }
    if (opts->low_pri.has) {
        rocksdb_writeoptions_set_low_pri(writer, opts->low_pri.v ? 1 : 0);
    }
    return writer;
}

db_t *db_open(const char *path, db_opts_t *cfg, char **err) {
    if (err) {
        *err = NULL;
    }
    if (path == NULL || strlen(path) == 0) {
        *err = strdup("path cannot be NULL");
        return NULL;
    }
    if (cfg == NULL) {
        *err = strdup("db config must not be NULL");
        return NULL;
    }
    if (cfg->cf_count == 0 || cfg->cf_opts == NULL) {
        *err = strdup("cf confgs must not be NULL");
        return NULL;
    }
    size_t cf_total = cfg->cf_count + 1;
    db_t *db = calloc(1, sizeof(*db));
    if (!db) {
        *err = strdup("oom");
        return NULL;
    }
    db->cf_count = cf_total;
    db->reader_count = cfg->reader_count;
    db->writer_count = cfg->writer_count;
    db->cf_handles = calloc(cf_total, sizeof *db->cf_handles);
    db->cf_names = calloc(cf_total, sizeof *db->cf_names);
    db->cf_options = calloc(cf_total, sizeof *db->cf_options);
    db->read_options = malloc(db->reader_count * sizeof(*db->read_options));
    db->write_options = malloc(db->writer_count * sizeof(*db->write_options));
    if (!db->cf_handles || !db->cf_names || !db->cf_options ||
        !db->read_options || !db->write_options) {
        *err = strdup("oom");
        return NULL;
    }
    if (cfg->block_cache_size.has) {
        db->shared_cache =
            rocksdb_cache_create_lru(cfg->block_cache_size.v);
        if (db->shared_cache == NULL) {
            *err = strdup("oom");
            return NULL;
        }
    }
    cf_opts_t default_opts = {.name = "default"};
    db->cf_names[0] = strdup(default_opts.name);
    db->cf_options[0] = cf_create(&default_opts, db->shared_cache);
    for (size_t i = 0; i < cfg->cf_count; i++) {
        db->cf_names[i + 1] = strdup(cfg->cf_opts[i]->name);
        db->cf_options[i + 1] = cf_create(cfg->cf_opts[i], db->shared_cache);
    }
    for (size_t i = 0; i < db->reader_count; i++) {
        db->read_options[i] = db_create_reader(cfg->reader_opts[i]);
    }
    for (size_t i = 0; i < db->writer_count; i++) {
        db->write_options[i] = db_create_writer(cfg->writer_opts[i]);
    }
    rocksdb_options_t *opts = rocksdb_options_create();
    rocksdb_options_set_create_if_missing(opts, 1);
    rocksdb_options_set_create_missing_column_families(opts, 1);
    db_apply_opts(opts, cfg);
    db->instance = rocksdb_open_column_families(
        opts, path, cf_total, db->cf_names,
        (const rocksdb_options_t *const *)db->cf_options, db->cf_handles, err);
    rocksdb_options_destroy(opts);
    return db;
}

void db_close(db_t *db) {
    if (db == NULL) {
        return;
    }
    if (db->cf_handles != NULL) {
        for (size_t i = 0; i < db->cf_count; i++) {
            rocksdb_column_family_handle_destroy(db->cf_handles[i]);
            db->cf_handles[i] = NULL;
        }
        free(db->cf_handles);
        db->cf_handles = NULL;
    }
    if (db->cf_names != NULL) {
        for (size_t i = 0; i < db->cf_count; i++) {
            free((void *)db->cf_names[i]);
            db->cf_names[i] = NULL;
        }
        free(db->cf_names);
        db->cf_names = NULL;
    }
    if (db->cf_options != NULL) {
        for (size_t i = 0; i < db->cf_count; i++) {
            rocksdb_options_destroy(db->cf_options[i]);
            db->cf_options[i] = NULL;
        }
        free(db->cf_options);
        db->cf_options = NULL;
    }
    if (db->read_options != NULL) {
        for (size_t i = 0; i < db->reader_count; i++) {
            rocksdb_readoptions_destroy(db->read_options[i]);
            db->read_options[i] = NULL;
        }
        free(db->read_options);
        db->read_options = NULL;
    }
    if (db->write_options != NULL) {
        for (size_t i = 0; i < db->writer_count; i++) {
            rocksdb_writeoptions_destroy(db->write_options[i]);
            db->write_options[i] = NULL;
        }
        free(db->write_options);
        db->write_options = NULL;
    }
    if (db->instance != NULL) {
        rocksdb_close(db->instance);
        db->instance = NULL;
    }
    if (db->shared_cache != NULL) {
        rocksdb_cache_destroy(db->shared_cache);
        db->shared_cache = NULL;
    }
    free(db);
}

void db_cf_drop_and_recreate(db_t *db, uint8_t cf_index, char **err) {
    db_is_valid(db, err);
    if (*err) {
        return;
    }
    if (cf_index == 0) {
        *err = strdup("invalid cf index: 0 is default, it can't be dropped");
        return;
    }
    db_cf_is_valid(db, cf_index, err);
    if (*err) {
        return;
    }
    rocksdb_drop_column_family(db->instance, db->cf_handles[cf_index], err);
    if (*err) {
        return;
    }
    rocksdb_column_family_handle_destroy(db->cf_handles[cf_index]);
    db->cf_handles[cf_index] = NULL;
    db->cf_handles[cf_index] = rocksdb_create_column_family(
        db->instance, db->cf_options[cf_index], db->cf_names[cf_index], err);
    if (*err) {
        db->cf_handles[cf_index] = NULL;
    }
}

void db_is_valid(db_t *db, char **err) {
    if (db == NULL || db->instance == NULL) {
        *err = strdup("db is not open");
    }
}

void db_reader_is_valid(db_t *db, uint8_t reader_index, char **err) {
    if (db == NULL || reader_index >= db->reader_count) {
        *err = strdup("reader index out of range");
    }
}

void db_writer_is_valid(db_t *db, uint8_t writer_index, char **err) {
    if (db == NULL || writer_index >= db->writer_count) {
        *err = strdup("writer index out of range");
    }
}

void db_cf_is_valid(db_t *db, uint8_t cf_index, char **err) {
    if (db == NULL || cf_index >= db->cf_count) {
        *err = strdup("cf index out of range");
    }
}
