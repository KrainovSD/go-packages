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

void print_bytes(const char *bytes, size_t bytes_len);
void print_bits(const void *ptr, size_t size);

#endif // STORAGE_ROCKSDB_CONNECTOR_HELPERS_H_
