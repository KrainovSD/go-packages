// Copyright 2026 KrainovSD
#include "stats.h"
#include "rocksdb_vendor/c.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

void db_stats(db_t *db, db_stats_t *out, char **err) {
    db_is_valid(db, err);
    if (*err) {
        return;
    }
    if (out == NULL) {
        *err = strdup("stats out must not be NULL");
        return;
    }
    rocksdb_t *instance = db->instance;
    rocksdb_property_int(instance, "rocksdb.background-errors",
                         &out->background_errors);
    rocksdb_property_int(instance, "rocksdb.num-running-compactions",
                         &out->num_running_compactions);
    rocksdb_property_int(instance, "rocksdb.num-running-flushes",
                         &out->num_running_flushes);
    rocksdb_property_int(instance, "rocksdb.num-unscheduled-compactions",
                         &out->num_unscheduled_compactions);
    rocksdb_property_int(instance, "rocksdb.is-write-stopped",
                         &out->is_write_stopped);
    rocksdb_property_int(instance, "rocksdb.num-snapshots",
                         &out->num_snapshots);
    rocksdb_property_int(instance, "rocksdb.oldest-snapshot-time",
                         &out->oldest_snapshot_time);
}

void db_cf_stats(db_t *db, uint8_t cf_index, cf_stats_t *out, char **err) {
    db_is_valid(db, err);
    if (*err) {
        return;
    }
    db_cf_is_valid(db, cf_index, err);
    if (*err) {
        return;
    }
    if (out == NULL) {
        *err = strdup("stats out must not be NULL");
        return;
    }
    rocksdb_t *instance = db->instance;
    rocksdb_column_family_handle_t *cf = db->cf_handles[cf_index];
    rocksdb_property_int_cf(instance, cf, "rocksdb.estimate-num-keys",
                            &out->estimate_num_keys);
    rocksdb_property_int_cf(instance, cf,
                            "rocksdb.num-entries-active-mem-table",
                            &out->num_entries_active_mem_table);
    rocksdb_property_int_cf(instance, cf, "rocksdb.num-entries-imm-mem-tables",
                            &out->num_entries_imm_mem_tables);
    rocksdb_property_int_cf(instance, cf,
                            "rocksdb.num-deletes-active-mem-table",
                            &out->num_deletes_active_mem_table);
    rocksdb_property_int_cf(instance, cf, "rocksdb.num-deletes-imm-mem-tables",
                            &out->num_deletes_imm_mem_tables);
    rocksdb_property_int_cf(instance, cf, "rocksdb.cur-size-active-mem-table",
                            &out->cur_size_active_mem_table);
    rocksdb_property_int_cf(instance, cf, "rocksdb.cur-size-all-mem-tables",
                            &out->cur_size_all_mem_tables);
    rocksdb_property_int_cf(instance, cf, "rocksdb.size-all-mem-tables",
                            &out->size_all_mem_tables);
    rocksdb_property_int_cf(instance, cf, "rocksdb.num-immutable-mem-table",
                            &out->num_immutable_mem_table);
    rocksdb_property_int_cf(instance, cf, "rocksdb.mem-table-flush-pending",
                            &out->mem_table_flush_pending);
    rocksdb_property_int_cf(instance, cf, "rocksdb.compaction-pending",
                            &out->compaction_pending);
    rocksdb_property_int_cf(instance, cf, "rocksdb.total-sst-files-size",
                            &out->total_sst_files_size);
    rocksdb_property_int_cf(instance, cf, "rocksdb.live-sst-files-size",
                            &out->live_sst_files_size);
    rocksdb_property_int_cf(instance, cf, "rocksdb.obsolete-sst-files-size",
                            &out->obsolete_sst_files_size);
    for (size_t level = 0; level < CF_STATS_LEVELS; level++) {
        char name[40];
        snprintf(name, sizeof(name), "rocksdb.num-files-at-level%zu", level);
        char *value = rocksdb_property_value_cf(instance, cf, name);
        if (value != NULL) {
            out->num_files_at_level[level] = strtoull(value, NULL, 10);
            free(value);
        }
    }
    rocksdb_property_int_cf(instance, cf, "rocksdb.estimate-table-readers-mem",
                            &out->estimate_table_readers_mem);
    rocksdb_property_int_cf(instance, cf,
                            "rocksdb.estimate-pending-compaction-bytes",
                            &out->estimate_pending_compaction_bytes);
    rocksdb_property_int_cf(instance, cf, "rocksdb.estimate-live-data-size",
                            &out->estimate_live_data_size);
    rocksdb_property_int_cf(instance, cf, "rocksdb.base-level",
                            &out->base_level);
    rocksdb_property_int_cf(instance, cf, "rocksdb.block-cache-usage",
                            &out->block_cache_usage);
    rocksdb_property_int_cf(instance, cf, "rocksdb.block-cache-pinned-usage",
                            &out->block_cache_pinned_usage);
    rocksdb_property_int_cf(instance, cf, "rocksdb.block-cache-capacity",
                            &out->block_cache_capacity);
    rocksdb_property_int_cf(instance, cf, "rocksdb.num-live-versions",
                            &out->num_live_versions);
}

void db_metrics(db_t *db, metrics_t *out, char **err) {
    db_is_valid(db, err);
    if (*err) {
        return;
    }
    if (out == NULL) {
        *err = strdup("metrics out must not be NULL");
        return;
    }
    if (!db->statistics_enabled) {
        *err = strdup("statistics is disabled");
        return;
    }
    rocksdb_options_t *opts = db->options;
    rocksdb_statistics_histogram_data_t *data =
        rocksdb_statistics_histogram_data_create();
    for (uint32_t ticker = 0; ticker < METRICS_TICKERS_COUNT; ticker++) {
        out->tickers[ticker] =
            rocksdb_options_statistics_get_ticker_count(opts, ticker);
    }
    for (uint32_t histogram = 0; histogram < METRICS_HISTOGRAMS_COUNT;
         histogram++) {
        rocksdb_options_statistics_get_histogram_data(opts, histogram, data);
        metrics_histogram_t *dst = &out->histograms[histogram];
        dst->count = rocksdb_statistics_histogram_data_get_count(data);
        dst->sum = rocksdb_statistics_histogram_data_get_sum(data);
        dst->min = rocksdb_statistics_histogram_data_get_min(data);
        dst->max = rocksdb_statistics_histogram_data_get_max(data);
        dst->median = rocksdb_statistics_histogram_data_get_median(data);
        dst->p95 = rocksdb_statistics_histogram_data_get_p95(data);
        dst->p99 = rocksdb_statistics_histogram_data_get_p99(data);
        dst->average = rocksdb_statistics_histogram_data_get_average(data);
        dst->std_dev = rocksdb_statistics_histogram_data_get_std_dev(data);
    }
    rocksdb_statistics_histogram_data_destroy(data);
}
