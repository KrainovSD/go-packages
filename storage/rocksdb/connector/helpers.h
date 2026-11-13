// Copyright 2026 KrainovSD
#ifndef STORAGE_ROCKSDB_CONNECTOR_HELPERS_H_
#define STORAGE_ROCKSDB_CONNECTOR_HELPERS_H_
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>

typedef struct {
    bool has;
    int v;
} opt_int_t;

typedef struct {
    bool has;
    size_t v;
} opt_size_t;

typedef struct {
    bool has;
    uint64_t v;
} opt_u64_t;

typedef struct {
    bool has;
    bool v;
} opt_bool_t;

typedef struct {
    bool has;
    unsigned int v;
} opt_unsigned_t;

typedef struct {
    bool has;
    double v;
} opt_double_t;

#define OPTS_KV_MAX 13
#define OPTS_KV_VALUE_MAX 256

typedef struct {
    const char *keys[OPTS_KV_MAX];
    const char *values[OPTS_KV_MAX];
    char buffers[OPTS_KV_MAX][OPTS_KV_VALUE_MAX];
    int count;
} opts_kv_t;

int opts_kv_add(opts_kv_t *kv, const char *key, const char *fmt, ...);
void opts_kv_add_int(opts_kv_t *kv, const char *key, opt_int_t opt);
void opts_kv_add_unsigned(opts_kv_t *kv, const char *key, opt_unsigned_t opt);
void opts_kv_add_u64(opts_kv_t *kv, const char *key, opt_u64_t opt);
void opts_kv_add_size(opts_kv_t *kv, const char *key, opt_size_t opt);
void opts_kv_add_double(opts_kv_t *kv, const char *key, opt_double_t opt);
void opts_kv_add_str(opts_kv_t *kv, const char *key, const char *value);

void print_bytes(const char *bytes, size_t bytes_len);
void print_bits(const void *ptr, size_t size);

#endif // STORAGE_ROCKSDB_CONNECTOR_HELPERS_H_
