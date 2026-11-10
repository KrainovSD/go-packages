// Copyright 2026 KrainovSD
#include "arena.h"
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static db_arena_chunk_t *db_arena_chunk_create(size_t size, size_t ttl,
                                               char **err) {
  db_arena_chunk_t *chunk = calloc(1, sizeof(*chunk));
  if (!chunk) {
    *err = strdup("oom");
    return NULL;
  }
  chunk->data = malloc(size);
  if (!chunk->data) {
    *err = strdup("oom");
    free(chunk);
    return NULL;
  }
  chunk->cap = size;
  chunk->ttl = ttl;
  return chunk;
}

static void db_arena_chunk_destroy(db_arena_chunk_t *chunk) {
  if (chunk == NULL) {
    return;
  }
  if (chunk->data != NULL) {
    free(chunk->data);
  }
  free(chunk);
}

db_arena_t *db_arena_create(size_t initial, size_t ttl, char **err) {
  db_arena_t *arena = calloc(1, sizeof(*arena));
  if (!arena) {
    *err = strdup("oom");
    return NULL;
  }
  db_arena_chunk_t *chunk = db_arena_chunk_create(initial, ttl, err);
  if (chunk == NULL) {
    free(arena);
    return NULL;
  }
  arena->chunks = malloc(sizeof(*arena->chunks));
  if (!arena->chunks) {
    db_arena_chunk_destroy(chunk);
    free(arena);
    *err = strdup("oom");
    return NULL;
  }
  arena->chunks[0] = chunk;
  arena->cap = 1;
  arena->ttl = ttl;
  arena->total_cap = initial;
  return arena;
}

void db_arena_destroy(db_arena_t *arena) {
  if (arena == NULL) {
    return;
  }
  if (arena->chunks != NULL) {
    for (size_t i = 0; i < arena->cap; i++) {
      db_arena_chunk_destroy(arena->chunks[i]);
    }
  }
  free(arena->chunks);
  free(arena);
}

static db_arena_chunk_t *db_arena_grow(db_arena_t *arena, size_t grow_size,
                                       char **err) {
  db_arena_chunk_t *chunk = db_arena_chunk_create(grow_size, arena->ttl, err);
  if (*err) {
    return NULL;
  }
  db_arena_chunk_t **chunks =
      realloc(arena->chunks, sizeof(*arena->chunks) * (arena->cap + 1));
  if (!chunks) {
    db_arena_chunk_destroy(chunk);
    *err = strdup("oom");
    return NULL;
  }
  arena->cap++;
  arena->total_cap += grow_size;
  arena->current = arena->cap - 1;
  chunks[arena->current] = chunk;
  arena->chunks = chunks;
  return chunk;
}

static db_arena_chunk_t *db_arena_next_chunk(db_arena_t *arena, char **err) {
  db_arena_chunk_t *chunk = NULL;
  if (arena->current == arena->cap - 1) {
    chunk = db_arena_grow(arena, arena->chunks[arena->current]->cap * 2, err);
    if (!chunk) {
      return NULL;
    }
  } else {
    arena->current++;
    chunk = arena->chunks[arena->current];
  }
  return chunk;
}

void db_arena_is_valid(db_arena_t *arena, char **err) {
  if (arena == NULL || arena->chunks == NULL || arena->cap == 0 ||
      arena->current >= arena->cap) {
    *err = strdup("arena is not valid");
  }
}

void db_arena_reset(db_arena_t *arena, char **err) {
  if (!arena) {
    return;
  }
  if (arena->chunks != NULL) {
    size_t kept = 0;
    for (size_t i = 0; i < arena->cap; i++) {
      db_arena_chunk_t *chunk = arena->chunks[i];
      bool unused = false;
      if (i > 0 && chunk->len == 0) {
        chunk->ttl--;
        unused = chunk->ttl <= 0;
      } else if (chunk->len != 0) {
        chunk->ttl = arena->ttl;
      }
      if (unused) {
        db_arena_chunk_destroy(chunk);
        arena->chunks[i] = NULL;
        continue;
      }
      chunk->len = 0;
      kept++;
    }
    if (kept < arena->cap) {
      size_t a = 0;
      arena->total_cap = 0;
      for (size_t i = 0; i < arena->cap; i++) {
        if (arena->chunks[i] != NULL) {
          arena->total_cap += arena->chunks[i]->cap;
          arena->chunks[a++] = arena->chunks[i];
        }
      }
      arena->cap = a;
    }
  }
  arena->current = 0;
}

char *db_arena_alloc(db_arena_t *arena, size_t value_len, char **err) {
  char *p = NULL;
  db_arena_chunk_t *chunk = arena->chunks[arena->current];
  while (chunk->cap - chunk->len < value_len) {
    chunk = db_arena_next_chunk(arena, err);
    if (!chunk) {
      return NULL;
    }
  }
  p = chunk->data + chunk->len;
  chunk->len += value_len;
  return p;
}

char *db_arena_pointer(db_arena_t *arena, char *value, size_t value_len,
                       char **err) {
  char *p = db_arena_alloc(arena, value_len, err);
  if (!p) {
    return NULL;
  }
  memcpy(p, value, value_len);
  return p;
}

void db_arena_append(db_arena_t *arena, char *value, size_t value_len,
                     char **err) {
  db_arena_chunk_t *chunk = arena->chunks[arena->current];
  while (chunk->cap - chunk->len < sizeof(size_t)) {
    chunk = db_arena_next_chunk(arena, err);
    if (!chunk) {
      return;
    }
  }
  memcpy(chunk->data + chunk->len, &value_len, sizeof(value_len));
  chunk->len += sizeof(value_len);
  while (value_len > 0) {
    size_t allowed = chunk->cap - chunk->len;
    if (allowed == 0) {
      chunk = db_arena_next_chunk(arena, err);
      if (!chunk) {
        return;
      }
      continue;
    }
    size_t write_len = value_len < allowed ? value_len : allowed;
    memcpy(chunk->data + chunk->len, value, write_len);
    value_len -= write_len;
    chunk->len += write_len;
  }
}
