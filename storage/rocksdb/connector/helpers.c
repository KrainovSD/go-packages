// Copyright 2026 KrainovSD
#include "helpers.h"
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

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
