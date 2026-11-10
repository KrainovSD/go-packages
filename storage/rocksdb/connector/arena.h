// Copyright 2026 KrainovSD
#ifndef STORAGE_ROCKSDB_CONNECTOR_ARENA_H_
#define STORAGE_ROCKSDB_CONNECTOR_ARENA_H_
#include "rocksdb_vendor/c.h"

typedef struct {
  char *data;
  size_t len;
  size_t cap;
  size_t ttl;
} db_arena_chunk_t;

typedef struct {
  db_arena_chunk_t **chunks;
  size_t current;
  size_t cap;
  size_t total_cap;
  size_t ttl;
} db_arena_t;

void db_arena_append(db_arena_t *arena, char *value, size_t value_len,
                     char **err);

db_arena_t *db_arena_create(size_t initial, size_t ttl, char **err);

void db_arena_destroy(db_arena_t *arena);

void db_arena_is_valid(db_arena_t *arena, char **err);

char *db_arena_alloc(db_arena_t *arena, size_t value_len, char **err);

char *db_arena_pointer(db_arena_t *arena, char *value, size_t value_len,
                       char **err);

void db_arena_reset(db_arena_t *arena, char **err);

#endif //  STORAGE_ROCKSDB_CONNECTOR_ARENA_H_
