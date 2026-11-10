// Copyright 2026 KrainovSD
#include "cf.h"
#include "rocksdb_vendor/c.h"
#include <assert.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

struct prefix_state_t {
    size_t offset_bytes;
    size_t len_bytes;
    char name[30];
};

static const char *generic_prefix_name(void *state) {
    struct prefix_state_t *s = (struct prefix_state_t *)state;
    return s->name;
}

static char *generic_prefix_transform(void *state, const char *key,
                                      size_t length, size_t *dst_length) {
    struct prefix_state_t *s = (struct prefix_state_t *)state;
    size_t total = s->len_bytes + s->offset_bytes;
    if (length < total) {
        *dst_length = 0;
        return NULL;
    }
    *dst_length = s->len_bytes;
    return (char *)key + s->offset_bytes;
}

static unsigned char generic_prefix_in_domain(void *state, const char *key,
                                              size_t length) {
    struct prefix_state_t *s = (struct prefix_state_t *)state;
    size_t total = s->len_bytes + s->offset_bytes;
    return (length >= total) ? 1 : 0;
}

static void generic_prefix_destructor(void *state) { free(state); }

static rocksdb_slicetransform_t *create_generic_prefix(size_t len,
                                                       size_t offset) {
    if (len == 0) {
        return NULL;
    }
    struct prefix_state_t *state = malloc(sizeof(*state));
    if (!state) {
        return NULL;
    }
    state->len_bytes = len;
    state->offset_bytes = offset;
    snprintf(state->name, sizeof(state->name), "prefix_%zuB_%zuO", len, offset);
    return rocksdb_slicetransform_create(
        state, &generic_prefix_destructor, &generic_prefix_transform,
        &generic_prefix_in_domain, &generic_prefix_name);
}

struct comparator_state_t {
    uint8_t type;
    char name[30];
};

static int generic_comparator_compare(void *state, const char *ap, size_t alen,
                                      const char *bp, size_t blen) {
    struct comparator_state_t *s = (struct comparator_state_t *)state;
    switch (s->type) {
    case CF_COMPARATOR_U64_LE: {
        uint64_t a = le64toh(*(uint64_t *)ap);
        uint64_t b = le64toh(*(uint64_t *)bp);
        return a == b ? 0 : a > b ? 1 : -1;
    }
    default: {
        size_t min_len = alen < blen ? alen : blen;
        int cmp = memcmp(ap, bp, min_len);
        if (cmp != 0) {
            return cmp;
        }
        if (alen < blen)
            return -1;
        if (alen > blen)
            return 1;
        return 0;
    }
    }
}

static const char *generic_comparator_name(void *state) {
    struct comparator_state_t *s = (struct comparator_state_t *)state;
    return s->name;
}

static void generic_comparator_destructor(void *state) { free(state); }

static rocksdb_comparator_t *
create_generic_comparator(uint8_t comparator_type) {
    if (comparator_type == 0) {
        return NULL;
    }
    struct comparator_state_t *state = malloc(sizeof(*state));
    if (!state) {
        return NULL;
    }
    state->type = comparator_type;
    snprintf(state->name, sizeof(state->name), "comparator_%u",
             (unsigned)comparator_type);
    return rocksdb_comparator_create(state, &generic_comparator_destructor,
                                     &generic_comparator_compare,
                                     &generic_comparator_name);
}

static void cf_apply_opts(rocksdb_options_t *opts, cf_opts_t *cfg) {
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

    if (cfg->write_buffer_size.has) {
        rocksdb_options_set_write_buffer_size(opts, cfg->write_buffer_size.v);
    }
    if (cfg->min_write_buffer_number_to_merge.has) {
        rocksdb_options_set_min_write_buffer_number_to_merge(
            opts, cfg->min_write_buffer_number_to_merge.v);
    }
    if (cfg->max_write_buffer_number.has) {
        rocksdb_options_set_max_write_buffer_number(
            opts, cfg->max_write_buffer_number.v);
    }

    if (cfg->target_file_size_base.has) {
        rocksdb_options_set_target_file_size_base(opts,
                                                  cfg->target_file_size_base.v);
    }
    if (cfg->target_file_size_multiplier.has) {
        rocksdb_options_set_target_file_size_multiplier(
            opts, cfg->target_file_size_multiplier.v);
    }
    if (cfg->max_bytes_for_level_base.has) {
        rocksdb_options_set_max_bytes_for_level_base(
            opts, cfg->max_bytes_for_level_base.v);
    }
    if (cfg->max_bytes_for_level_multiplier.has) {
        rocksdb_options_set_max_bytes_for_level_multiplier(
            opts, cfg->max_bytes_for_level_multiplier.v);
    }
    if (cfg->level_compaction_dynamic_level_bytes.has) {
        rocksdb_options_set_level_compaction_dynamic_level_bytes(
            opts, cfg->level_compaction_dynamic_level_bytes.v ? 1 : 0);
    }

    if (cfg->level0_file_num_compaction_trigger.has) {
        rocksdb_options_set_level0_file_num_compaction_trigger(
            opts, cfg->level0_file_num_compaction_trigger.v);
    }
    if (cfg->level0_slowdown_writes_trigger.has) {
        rocksdb_options_set_level0_slowdown_writes_trigger(
            opts, cfg->level0_slowdown_writes_trigger.v);
    }
    if (cfg->level0_stop_writes_trigger.has) {
        rocksdb_options_set_level0_stop_writes_trigger(
            opts, cfg->level0_stop_writes_trigger.v);
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

    if (cfg->ttl.has) {
        rocksdb_options_set_ttl(opts, cfg->ttl.v);
    }
}

static void cf_set_block_based_table_opts(rocksdb_options_t *opts,
                                          cf_table_opts_t *cfg) {
    if (cfg == NULL) {
        return;
    }
    rocksdb_block_based_table_options_t *table_opts =
        rocksdb_block_based_options_create();
    if (cfg->bloom_bits_per_key.has) {
        rocksdb_block_based_options_set_filter_policy(
            table_opts,
            rocksdb_filterpolicy_create_bloom_full(cfg->bloom_bits_per_key.v));
        if (cfg->bloom_whole_key_filering.has) {
            rocksdb_block_based_options_set_whole_key_filtering(
                table_opts, cfg->bloom_whole_key_filering.v ? 1 : 0);
        }
    }
    rocksdb_cache_t *cache = NULL;
    if (cfg->block_cache_size.has) {
        cache = rocksdb_cache_create_lru(cfg->block_cache_size.v);
        rocksdb_block_based_options_set_block_cache(table_opts, cache);
    }

    rocksdb_options_set_block_based_table_factory(opts, table_opts);
    rocksdb_block_based_options_destroy(table_opts);
    if (cache != NULL) {
        rocksdb_cache_destroy(cache);
    }
}

rocksdb_options_t *cf_create(cf_opts_t *cfg) {
    rocksdb_options_t *opts = rocksdb_options_create();
    cf_apply_opts(opts, cfg);
    cf_set_block_based_table_opts(opts, cfg->table_opts);
    if (cfg->merge_operator != NULL) {
        rocksdb_options_set_merge_operator(opts, cfg->merge_operator);
    }
    rocksdb_slicetransform_t *prefixer =
        create_generic_prefix(cfg->prefix_len, cfg->prefix_offset);
    if (prefixer != NULL) {
        rocksdb_options_set_prefix_extractor(opts, prefixer);
    }
    rocksdb_comparator_t *comparator =
        create_generic_comparator(cfg->comparator_type);
    if (comparator != NULL) {
        rocksdb_options_set_comparator(opts, comparator);
    }
    return opts;
}
