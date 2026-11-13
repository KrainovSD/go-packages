// Copyright 2026 KrainovSD
#include "helpers.h"
#include <assert.h>
#include <inttypes.h>
#include <stdarg.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

int opts_kv_add(opts_kv_t *kv, const char *key, const char *fmt, ...) {
    if (kv->count >= OPTS_KV_MAX) {
        return -1;
    }
    va_list args;
    va_start(args, fmt);
    int written =
        vsnprintf(kv->buffers[kv->count], OPTS_KV_VALUE_MAX, fmt, args);
    va_end(args);
    if (written < 0 || written >= OPTS_KV_VALUE_MAX) {
        return -1;
    }
    kv->keys[kv->count] = key;
    kv->values[kv->count] = kv->buffers[kv->count];
    kv->count++;
    return 0;
}

void opts_kv_add_int(opts_kv_t *kv, const char *key, opt_int_t opt) {
    if (opt.has) {
        assert(opts_kv_add(kv, key, "%d", opt.v) == 0);
    }
}

void opts_kv_add_unsigned(opts_kv_t *kv, const char *key, opt_unsigned_t opt) {
    if (opt.has) {
        assert(opts_kv_add(kv, key, "%u", opt.v) == 0);
    }
}

void opts_kv_add_u64(opts_kv_t *kv, const char *key, opt_u64_t opt) {
    if (opt.has) {
        assert(opts_kv_add(kv, key, "%" PRIu64, opt.v) == 0);
    }
}

void opts_kv_add_size(opts_kv_t *kv, const char *key, opt_size_t opt) {
    if (opt.has) {
        assert(opts_kv_add(kv, key, "%zu", opt.v) == 0);
    }
}

void opts_kv_add_double(opts_kv_t *kv, const char *key, opt_double_t opt) {
    if (opt.has) {
        assert(opts_kv_add(kv, key, "%.17g", opt.v) == 0);
    }
}

void opts_kv_add_str(opts_kv_t *kv, const char *key, const char *value) {
    assert(opts_kv_add(kv, key, "%s", value) == 0);
}

void print_bytes(const char *bytes, size_t bytes_len) {
    for (size_t i = 0; i < bytes_len; i++) {
        printf("%02x ", (unsigned char)bytes[i]);
    }
    printf("\n");
}

void print_bits(const void *ptr, size_t size) {
    const uint8_t *bytes = (const uint8_t *)ptr;

    printf("[");
    for (size_t i = 0; i < size; i++) {
        for (int j = 7; j >= 0; j--) {
            printf("%d", (bytes[i] >> j) & 1);
        }

        if (i < size - 1)
            printf(" ");
    }
    printf("]\n");

    return;
}
