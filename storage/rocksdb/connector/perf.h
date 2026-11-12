// Copyright 2026 KrainovSD
#ifndef STORAGE_ROCKSDB_CONNECTOR_PERF_H_
#define STORAGE_ROCKSDB_CONNECTOR_PERF_H_
#include "rocksdb_vendor/c.h"
#include <stdint.h>

#define PERF_METRICS_COUNT 84

typedef struct {
    rocksdb_perfcontext_t *ctx;
} perf_t;

typedef struct {
    uint64_t values[PERF_METRICS_COUNT];
} perf_metrics_t;

void perf_open(perf_t *perf, int level, char **err);

void perf_metrics(perf_t *perf, perf_metrics_t *out, char **err);

void perf_close(perf_t *perf);

#endif // STORAGE_ROCKSDB_CONNECTOR_PERF_H_
