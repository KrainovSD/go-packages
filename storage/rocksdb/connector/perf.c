// Copyright 2026 KrainovSD
#include "perf.h"
#include <string.h>

void perf_open(perf_t *perf, int level, char **err) {
    if (perf == NULL) {
        *err = strdup("perf must not be NULL");
        return;
    }
    if (level < 1 || level > 6) {
        *err = strdup("perf level must be in range 1-6");
        return;
    }
    perf->ctx = rocksdb_perfcontext_create();
    rocksdb_set_perf_level(level);
    rocksdb_perfcontext_reset(perf->ctx);
}

void perf_metrics(perf_t *perf, perf_metrics_t *out, char **err) {
    if (perf == NULL || perf->ctx == NULL) {
        *err = strdup("perf context is not open");
        return;
    }
    if (out == NULL) {
        *err = strdup("perf metrics out must not be NULL");
        return;
    }
    for (uint32_t metric = 0; metric < PERF_METRICS_COUNT; metric++) {
        out->values[metric] =
            rocksdb_perfcontext_metric(perf->ctx, (int)metric);
    }
}

void perf_close(perf_t *perf) {
    if (perf == NULL || perf->ctx == NULL) {
        return;
    }
    rocksdb_perfcontext_destroy(perf->ctx);
    perf->ctx = NULL;
    rocksdb_set_perf_level(rocksdb_enable_count);
}
