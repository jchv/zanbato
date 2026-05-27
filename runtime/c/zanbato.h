#ifndef ZB_ZANBATO_H_
#define ZB_ZANBATO_H_

#include <errno.h>
#include <iconv.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <zlib.h>

#ifdef __cplusplus
extern "C" {
#endif

#define ZB_OK 0
#define ZB_ERR_EOF -1
#define ZB_ERR_IO -2
#define ZB_ERR_ALLOC -3
#define ZB_ERR_VALIDATION -4
#define ZB_ERR_FORMAT -5
#define ZB_ERR_UNSUPPORTED -6
#define ZB_ERR_OVERFLOW -7

#ifdef __GNUC__
#undef ZB_UNUSED
#define ZB_UNUSED __attribute__((unused))
#else
#define ZB_UNUSED
#endif

#define ZB_NEW(arena, T, lvalue)                                               \
  do {                                                                         \
    (lvalue) = (T *)zb_arena_calloc((arena), 1, sizeof(T));                    \
    if (!(lvalue)) {                                                           \
      return ZB_ERR_ALLOC;                                                     \
    }                                                                          \
  } while (0)

#define ZB_TRY(call)                                                           \
  do {                                                                         \
    int _e = (call);                                                           \
    if (_e) {                                                                  \
      return _e;                                                               \
    }                                                                          \
  } while (0)

typedef struct zb_arena_chunk {
  struct zb_arena_chunk *next;
  size_t used;
  size_t cap;
} zb_arena_chunk_t;

typedef struct zb_arena {
  zb_arena_chunk_t *head;
  size_t default_chunk;
} zb_arena_t;

#define ZB_ARENA_DEFAULT_CHUNK ((size_t)4096)

static ZB_UNUSED void zb_arena_init(zb_arena_t *a) {
  a->head = NULL;
  a->default_chunk = ZB_ARENA_DEFAULT_CHUNK;
}

static ZB_UNUSED void zb_arena_destroy(zb_arena_t *a) {
  zb_arena_chunk_t *c = a->head;
  while (c) {
    zb_arena_chunk_t *next = c->next;
    free(c);
    c = next;
  }
  a->head = NULL;
}

static ZB_UNUSED void *zb_arena_alloc(zb_arena_t *a, size_t size) {
  if (size == 0) {
    size = 1;
  }
  /* round up to 8-byte alignment */
  size = (size + 7u) & ~(size_t)7u;
  zb_arena_chunk_t *c = a->head;
  if (!c || c->used + size > c->cap) {
    size_t cap = a->default_chunk ? a->default_chunk : ZB_ARENA_DEFAULT_CHUNK;
    if (cap < size) {
      cap = size;
    }
    zb_arena_chunk_t *nc =
        (zb_arena_chunk_t *)malloc(sizeof(zb_arena_chunk_t) + cap);
    if (!nc) {
      return NULL;
    }
    nc->next = a->head;
    nc->used = 0;
    nc->cap = cap;
    a->head = nc;
    c = nc;
  }
  char *base = (char *)(c + 1);
  void *p = base + c->used;
  c->used += size;
  return p;
}

static ZB_UNUSED void *zb_arena_calloc(zb_arena_t *a, size_t n, size_t size) {
  size_t total = n * size;
  if (n != 0 && total / n != size) {
    return NULL; /* overflow */
  }
  void *p = zb_arena_alloc(a, total);
  if (p) {
    memset(p, 0, total);
  }
  return p;
}

static ZB_UNUSED char *zb_arena_strdup(zb_arena_t *a, const char *s) {
  size_t n = strlen(s);
  char *out = (char *)zb_arena_alloc(a, n + 1);
  if (!out) {
    return NULL;
  }
  memcpy(out, s, n + 1);
  return out;
}

static ZB_UNUSED void *zb_arena_dup(zb_arena_t *a, const void *src, size_t n) {
  if (n == 0) {
    return NULL;
  }
  void *out = zb_arena_alloc(a, n);
  if (!out) {
    return NULL;
  }
  memcpy(out, src, n);
  return out;
}

typedef struct zb_bytes {
  const uint8_t *data;
  size_t len;
} zb_bytes_t;

static ZB_UNUSED zb_bytes_t zb_bytes_make(const uint8_t *data, size_t len) {
  zb_bytes_t b;
  b.data = data;
  b.len = len;
  return b;
}

static ZB_UNUSED int zb_bytes_equal(zb_bytes_t a, zb_bytes_t b) {
  if (a.len != b.len) {
    return 0;
  }
  if (a.len == 0) {
    return 1;
  }
  return memcmp(a.data, b.data, a.len) == 0;
}

static ZB_UNUSED zb_bytes_t zb_bytes_dup(zb_arena_t *arena, zb_bytes_t b) {
  if (b.len == 0) {
    return zb_bytes_make(NULL, 0);
  }
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, b.len);
  memcpy(out, b.data, b.len);
  return zb_bytes_make(out, b.len);
}

static ZB_UNUSED zb_bytes_t zb_bytes_concat(zb_arena_t *arena, zb_bytes_t a,
                                            zb_bytes_t b) {
  size_t n = a.len + b.len;
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, n ? n : 1);
  if (a.len) {
    memcpy(out, a.data, a.len);
  }
  if (b.len) {
    memcpy(out + a.len, b.data, b.len);
  }
  return zb_bytes_make(out, n);
}

static ZB_UNUSED int zb_bytes_compare(zb_bytes_t a, zb_bytes_t b) {
  size_t n = a.len < b.len ? a.len : b.len;
  int c = n ? memcmp(a.data, b.data, n) : 0;
  if (c != 0) {
    return c;
  }
  if (a.len < b.len) {
    return -1;
  }
  if (a.len > b.len) {
    return 1;
  }
  return 0;
}

static ZB_UNUSED zb_bytes_t zb_bytes_slice(zb_bytes_t b, size_t from,
                                           size_t to) {
  if (from > b.len) {
    from = b.len;
  }
  if (to > b.len) {
    to = b.len;
  }
  if (to < from) {
    to = from;
  }
  return zb_bytes_make(b.data + from, to - from);
}

static ZB_UNUSED zb_bytes_t zb_bytes_strip_right(zb_bytes_t b, int pad_byte) {
  if (pad_byte < 0) {
    return b;
  }
  size_t n = b.len;
  while (n > 0 && b.data[n - 1] == (uint8_t)pad_byte) {
    n--;
  }
  return zb_bytes_make(b.data, n);
}

static ZB_UNUSED ptrdiff_t zb_bytes_index(zb_bytes_t b, uint8_t needle) {
  for (size_t i = 0; i < b.len; i++) {
    if (b.data[i] == needle) {
      return (ptrdiff_t)i;
    }
  }
  return -1;
}

static ZB_UNUSED ptrdiff_t zb_bytes_index_multi(zb_bytes_t b,
                                                const uint8_t *needle,
                                                size_t unit) {
  if (unit == 0 || b.len < unit) {
    return -1;
  }
  for (size_t i = 0; i + unit <= b.len; i += unit) {
    if (memcmp(b.data + i, needle, unit) == 0) {
      return (ptrdiff_t)i;
    }
  }
  return -1;
}

static ZB_UNUSED zb_bytes_t zb_bytes_terminate_multi(zb_bytes_t b,
                                                     const uint8_t *term,
                                                     size_t unit, int include) {
  ptrdiff_t i = zb_bytes_index_multi(b, term, unit);
  if (i < 0) {
    return b;
  }
  size_t end = (size_t)i + (include ? unit : 0);
  return zb_bytes_make(b.data, end);
}

typedef struct zb_buf {
  zb_arena_t *arena;
  uint8_t *data;
  size_t len;
  size_t cap;
} zb_buf_t;

static ZB_UNUSED void zb_buf_init(zb_buf_t *b, zb_arena_t *arena) {
  b->arena = arena;
  b->data = NULL;
  b->len = 0;
  b->cap = 0;
}

static ZB_UNUSED int zb_buf_reserve(zb_buf_t *b, size_t additional) {
  size_t need = b->len + additional;
  if (need <= b->cap) {
    return ZB_OK;
  }
  size_t new_cap = b->cap ? b->cap : 64;
  while (new_cap < need) {
    new_cap *= 2;
  }
  uint8_t *nd = (uint8_t *)zb_arena_alloc(b->arena, new_cap);
  if (!nd) {
    return ZB_ERR_ALLOC;
  }
  if (b->len) {
    memcpy(nd, b->data, b->len);
  }
  b->data = nd;
  b->cap = new_cap;
  return ZB_OK;
}

static ZB_UNUSED int zb_buf_append(zb_buf_t *b, const uint8_t *src, size_t n) {
  int err = zb_buf_reserve(b, n);
  if (err) {
    return err;
  }
  if (n) {
    memcpy(b->data + b->len, src, n);
  }
  b->len += n;
  return ZB_OK;
}

static ZB_UNUSED int zb_buf_append_byte(zb_buf_t *b, uint8_t v) {
  return zb_buf_append(b, &v, 1);
}

static ZB_UNUSED zb_bytes_t zb_buf_to_bytes(const zb_buf_t *b) {
  return zb_bytes_make(b->data, b->len);
}

#define ZB_ARRAY(ELEM_T)                                                       \
  struct {                                                                     \
    ELEM_T *data;                                                              \
    size_t len;                                                                \
    size_t cap;                                                                \
  }

static ZB_UNUSED int zb_array_grow_impl(zb_arena_t *arena, void **data,
                                        size_t *cap, size_t need,
                                        size_t elem_size) {
  if (need <= *cap) {
    return ZB_OK;
  }
  size_t new_cap = *cap ? *cap : 8;
  while (new_cap < need) {
    new_cap *= 2;
  }
  void *nd = zb_arena_alloc(arena, new_cap * elem_size);
  if (!nd) {
    return ZB_ERR_ALLOC;
  }
  if (*data && *cap) {
    memcpy(nd, *data, *cap * elem_size);
  }
  *data = nd;
  *cap = new_cap;
  return ZB_OK;
}

#define zb_array_push(arena, arr, elem)                                        \
  (zb_array_grow_impl((arena), (void **)&(arr).data, &(arr).cap,               \
                      (arr).len + 1, sizeof(*(arr).data)) == ZB_OK             \
       ? ((arr).data[(arr).len++] = (elem), ZB_OK)                             \
       : ZB_ERR_ALLOC)

typedef struct zb_debug_arr_pos {
  int64_t start;
  int64_t end;
} zb_debug_arr_pos_t;

typedef struct zb_debug_pos {
  const char *name;
  int64_t start;
  int64_t end;
  zb_debug_arr_pos_t *arr;
  size_t arr_len;
  size_t arr_cap;
} zb_debug_pos_t;

typedef struct zb_debug_info {
  zb_debug_pos_t *entries;
  size_t len;
  size_t cap;
} zb_debug_info_t;

static ZB_UNUSED zb_debug_pos_t *
zb_debug_get_or_add(zb_debug_info_t *d, zb_arena_t *arena, const char *name) {
  for (size_t i = 0; i < d->len; i++) {
    if (d->entries[i].name == name || strcmp(d->entries[i].name, name) == 0) {
      return &d->entries[i];
    }
  }
  if (zb_array_grow_impl(arena, (void **)&d->entries, &d->cap, d->len + 1,
                         sizeof(*d->entries)) != ZB_OK) {
    return NULL;
  }
  zb_debug_pos_t *p = &d->entries[d->len++];
  memset(p, 0, sizeof(*p));
  p->name = name;
  p->start = -1;
  p->end = -1;
  return p;
}

static ZB_UNUSED zb_debug_pos_t *zb_debug_lookup(const zb_debug_info_t *d,
                                                 const char *name) {
  if (!d) {
    return NULL;
  }
  for (size_t i = 0; i < d->len; i++) {
    if (d->entries[i].name == name || strcmp(d->entries[i].name, name) == 0) {
      return (zb_debug_pos_t *)&d->entries[i];
    }
  }
  return NULL;
}

static ZB_UNUSED void zb_debug_attr_start(zb_debug_info_t *d, zb_arena_t *arena,
                                          const char *name, int64_t pos) {
  zb_debug_pos_t *p = zb_debug_get_or_add(d, arena, name);
  if (p) {
    p->start = pos;
  }
}

static ZB_UNUSED void zb_debug_attr_end(zb_debug_info_t *d, zb_arena_t *arena,
                                        const char *name, int64_t pos) {
  zb_debug_pos_t *p = zb_debug_get_or_add(d, arena, name);
  if (p) {
    p->end = pos;
  }
}

static ZB_UNUSED void zb_debug_arr_init(zb_debug_info_t *d, zb_arena_t *arena,
                                        const char *name) {
  zb_debug_pos_t *p = zb_debug_get_or_add(d, arena, name);
  if (p) {
    p->arr_len = 0;
  }
}

static ZB_UNUSED void zb_debug_arr_elem_start(zb_debug_info_t *d,
                                              zb_arena_t *arena,
                                              const char *name, int64_t pos) {
  zb_debug_pos_t *p = zb_debug_get_or_add(d, arena, name);
  if (!p) {
    return;
  }
  if (zb_array_grow_impl(arena, (void **)&p->arr, &p->arr_cap, p->arr_len + 1,
                         sizeof(*p->arr)) != ZB_OK) {
    return;
  }
  p->arr[p->arr_len].start = pos;
  p->arr[p->arr_len].end = -1;
  p->arr_len++;
}

static ZB_UNUSED void zb_debug_arr_elem_end(zb_debug_info_t *d,
                                            zb_arena_t *arena, const char *name,
                                            int64_t pos) {
  zb_debug_pos_t *p = zb_debug_get_or_add(d, arena, name);
  if (!p || p->arr_len == 0) {
    return;
  }
  p->arr[p->arr_len - 1].end = pos;
}

typedef struct zb_stream {
  const uint8_t *mem;
  size_t mem_len;
  FILE *file;
  int owns_file;

  size_t base;
  size_t limit;

  size_t pos;

  uint64_t bits;
  int bits_left;
  int bits_le;
} zb_stream_t;

static ZB_UNUSED void zb_stream_init_mem(zb_stream_t *s, const void *data,
                                         size_t len) {
  memset(s, 0, sizeof(*s));
  s->mem = (const uint8_t *)data;
  s->mem_len = len;
  s->limit = len;
}

static ZB_UNUSED int zb_stream_init_file(zb_stream_t *s, FILE *f,
                                         int owns_file) {
  memset(s, 0, sizeof(*s));
  s->file = f;
  s->owns_file = owns_file;
  if (fseek(f, 0, SEEK_END) != 0) {
    return ZB_ERR_IO;
  }
  long end = ftell(f);
  if (end < 0) {
    return ZB_ERR_IO;
  }
  if (fseek(f, 0, SEEK_SET) != 0) {
    return ZB_ERR_IO;
  }
  s->limit = (size_t)end;
  return ZB_OK;
}

static ZB_UNUSED int zb_stream_open(zb_stream_t *s, const char *path) {
  FILE *f = fopen(path, "rb");
  if (!f) {
    return ZB_ERR_IO;
  }
  int err = zb_stream_init_file(s, f, 1);
  if (err) {
    fclose(f);
  }
  return err;
}

static ZB_UNUSED void zb_stream_close(zb_stream_t *s) {
  if (s->file && s->owns_file) {
    fclose(s->file);
  }
  memset(s, 0, sizeof(*s));
}

static ZB_UNUSED void zb_stream_substream(const zb_stream_t *parent,
                                          zb_stream_t *out, size_t off,
                                          size_t len) {
  memset(out, 0, sizeof(*out));
  out->mem = parent->mem;
  out->mem_len = parent->mem_len;
  out->file = parent->file;
  out->owns_file = 0;
  size_t base = parent->base + off;
  if (base > parent->base + parent->limit) {
    base = parent->base + parent->limit;
  }
  out->base = base;
  if (base + len > parent->base + parent->limit) {
    len = parent->base + parent->limit - base;
  }
  out->limit = len;
  out->pos = 0;
}

static ZB_UNUSED void zb_stream_init_substream_bytes(zb_stream_t *out,
                                                     zb_bytes_t b) {
  zb_stream_init_mem(out, b.data, b.len);
}

static ZB_UNUSED zb_stream_t *zb_substream_mem(zb_arena_t *arena,
                                               const void *data, size_t len) {
  zb_stream_t *s = (zb_stream_t *)zb_arena_alloc(arena, sizeof(zb_stream_t));
  if (!s) {
    return NULL;
  }
  zb_stream_init_mem(s, data, len);
  return s;
}

static ZB_UNUSED zb_stream_t *zb_substream_view(zb_arena_t *arena,
                                                const zb_stream_t *parent,
                                                size_t start, size_t len) {
  zb_stream_t *s = (zb_stream_t *)zb_arena_alloc(arena, sizeof(zb_stream_t));
  if (!s) {
    return NULL;
  }
  zb_stream_substream(parent, s, start, len);
  return s;
}

static ZB_UNUSED zb_bytes_t *zb_bytes_box(zb_arena_t *arena, zb_bytes_t b) {
  zb_bytes_t *out = (zb_bytes_t *)zb_arena_alloc(arena, sizeof(zb_bytes_t));
  if (!out) {
    return NULL;
  }
  *out = b;
  return out;
}

static ZB_UNUSED size_t zb_stream_size(const zb_stream_t *s) {
  return s->limit;
}
static ZB_UNUSED size_t zb_stream_pos(const zb_stream_t *s) { return s->pos; }
static ZB_UNUSED int zb_stream_eof(const zb_stream_t *s) {
  return s->pos >= s->limit && s->bits_left == 0;
}

static ZB_UNUSED int zb_stream_seek(zb_stream_t *s, size_t pos) {
  if (pos > s->limit) {
    return ZB_ERR_EOF;
  }
  s->pos = pos;
  s->bits = 0;
  s->bits_left = 0;
  return ZB_OK;
}

static ZB_UNUSED void zb_stream_align_to_byte(zb_stream_t *s) {
  s->bits = 0;
  s->bits_left = 0;
}

static ZB_UNUSED int zb_stream_read_raw(zb_stream_t *s, uint8_t *out,
                                        size_t n) {
  s->bits = 0;
  s->bits_left = 0;
  if (n == 0) {
    return ZB_OK;
  }
  if (s->pos + n > s->limit) {
    return ZB_ERR_EOF;
  }
  if (s->mem) {
    memcpy(out, s->mem + s->base + s->pos, n);
  } else if (s->file) {
    if (fseek(s->file, (long)(s->base + s->pos), SEEK_SET) != 0) {
      return ZB_ERR_IO;
    }
    if (fread(out, 1, n, s->file) != n) {
      return ZB_ERR_IO;
    }
  } else {
    return ZB_ERR_IO;
  }
  s->pos += n;
  return ZB_OK;
}

static ZB_UNUSED int zb_stream_read_raw_keep_bits(zb_stream_t *s, uint8_t *out,
                                                  size_t n) {
  if (n == 0) {
    return ZB_OK;
  }
  if (s->pos + n > s->limit) {
    return ZB_ERR_EOF;
  }
  if (s->mem) {
    memcpy(out, s->mem + s->base + s->pos, n);
  } else if (s->file) {
    if (fseek(s->file, (long)(s->base + s->pos), SEEK_SET) != 0) {
      return ZB_ERR_IO;
    }
    if (fread(out, 1, n, s->file) != n) {
      return ZB_ERR_IO;
    }
  } else {
    return ZB_ERR_IO;
  }
  s->pos += n;
  return ZB_OK;
}

static ZB_UNUSED int zb_read_u1(zb_stream_t *s, uint8_t *out) {
  return zb_stream_read_raw(s, out, 1);
}

static ZB_UNUSED int zb_read_s1(zb_stream_t *s, int8_t *out) {
  return zb_stream_read_raw(s, (uint8_t *)out, 1);
}

#define ZB_DEFINE_READ_INT(NAME, T, N, BE)                                     \
  static ZB_UNUSED int NAME(zb_stream_t *s, T *out) {                          \
    uint8_t buf[N];                                                            \
    int err = zb_stream_read_raw(s, buf, N);                                   \
    if (err) {                                                                 \
      return err;                                                              \
    }                                                                          \
    uint64_t v = 0;                                                            \
    if (BE) {                                                                  \
      for (int i = 0; i < N; i++) {                                            \
        v = (v << 8) | buf[i];                                                 \
      }                                                                        \
    } else {                                                                   \
      for (int i = N - 1; i >= 0; i--) {                                       \
        v = (v << 8) | buf[i];                                                 \
      }                                                                        \
    }                                                                          \
    *out = (T)v;                                                               \
    return ZB_OK;                                                              \
  }

ZB_DEFINE_READ_INT(zb_read_u2be, uint16_t, 2, 1)
ZB_DEFINE_READ_INT(zb_read_u2le, uint16_t, 2, 0)
ZB_DEFINE_READ_INT(zb_read_u4be, uint32_t, 4, 1)
ZB_DEFINE_READ_INT(zb_read_u4le, uint32_t, 4, 0)
ZB_DEFINE_READ_INT(zb_read_u8be, uint64_t, 8, 1)
ZB_DEFINE_READ_INT(zb_read_u8le, uint64_t, 8, 0)
ZB_DEFINE_READ_INT(zb_read_s2be, int16_t, 2, 1)
ZB_DEFINE_READ_INT(zb_read_s2le, int16_t, 2, 0)
ZB_DEFINE_READ_INT(zb_read_s4be, int32_t, 4, 1)
ZB_DEFINE_READ_INT(zb_read_s4le, int32_t, 4, 0)
ZB_DEFINE_READ_INT(zb_read_s8be, int64_t, 8, 1)
ZB_DEFINE_READ_INT(zb_read_s8le, int64_t, 8, 0)

#undef ZB_DEFINE_READ_INT

static ZB_UNUSED int zb_read_f4be(zb_stream_t *s, float *out) {
  uint32_t u;
  int err = zb_read_u4be(s, &u);
  if (err) {
    return err;
  }
  memcpy(out, &u, 4);
  return ZB_OK;
}
static ZB_UNUSED int zb_read_f4le(zb_stream_t *s, float *out) {
  uint32_t u;
  int err = zb_read_u4le(s, &u);
  if (err) {
    return err;
  }
  memcpy(out, &u, 4);
  return ZB_OK;
}
static ZB_UNUSED int zb_read_f8be(zb_stream_t *s, double *out) {
  uint64_t u;
  int err = zb_read_u8be(s, &u);
  if (err) {
    return err;
  }
  memcpy(out, &u, 8);
  return ZB_OK;
}
static ZB_UNUSED int zb_read_f8le(zb_stream_t *s, double *out) {
  uint64_t u;
  int err = zb_read_u8le(s, &u);
  if (err) {
    return err;
  }
  memcpy(out, &u, 8);
  return ZB_OK;
}

static ZB_UNUSED int zb_read_bits_be(zb_stream_t *s, int n, uint64_t *out) {
  if (n < 0 || n > 64) {
    return ZB_ERR_OVERFLOW;
  }
  if (s->bits_le) {
    s->bits = 0;
    s->bits_left = 0;
  }
  s->bits_le = 0;

  uint64_t result = 0;
  int collected = 0;

  if (s->bits_left >= n) {
    int shift = s->bits_left - n;
    uint64_t mask = (n == 64) ? ~(uint64_t)0 : ((uint64_t)1 << n) - 1;
    *out = (s->bits >> shift) & mask;
    s->bits_left -= n;
    if (s->bits_left == 0) {
      s->bits = 0;
    } else {
      s->bits &= ((uint64_t)1 << s->bits_left) - 1;
    }
    return ZB_OK;
  }
  if (s->bits_left > 0) {
    result = s->bits & (((uint64_t)1 << s->bits_left) - 1);
    collected = s->bits_left;
    s->bits = 0;
    s->bits_left = 0;
  }
  while (collected + 8 <= n) {
    uint8_t b;
    int err = zb_stream_read_raw_keep_bits(s, &b, 1);
    if (err) {
      return err;
    }
    result = (result << 8) | b;
    collected += 8;
  }
  if (collected < n) {
    uint8_t b;
    int err = zb_stream_read_raw_keep_bits(s, &b, 1);
    if (err) {
      return err;
    }
    int need = n - collected;
    result = (result << need) | ((uint64_t)b >> (8 - need));
    s->bits = b & (((uint64_t)1 << (8 - need)) - 1);
    s->bits_left = 8 - need;
  }
  *out = result;
  return ZB_OK;
}

static ZB_UNUSED int zb_read_bits_le(zb_stream_t *s, int n, uint64_t *out) {
  if (n < 0 || n > 64) {
    return ZB_ERR_OVERFLOW;
  }
  if (!s->bits_le) {
    s->bits = 0;
    s->bits_left = 0;
  }
  s->bits_le = 1;

  uint64_t result = 0;
  int collected = 0;

  int take = (s->bits_left < n) ? s->bits_left : n;
  if (take > 0) {
    uint64_t mask = (take == 64) ? ~(uint64_t)0 : ((uint64_t)1 << take) - 1;
    result = s->bits & mask;
    s->bits >>= take;
    s->bits_left -= take;
    collected = take;
  }
  while (collected < n) {
    uint8_t b;
    int err = zb_stream_read_raw_keep_bits(s, &b, 1);
    if (err) {
      return err;
    }
    int wanted = n - collected;
    if (wanted >= 8) {
      result |= ((uint64_t)b) << collected;
      collected += 8;
    } else {
      uint64_t mask = ((uint64_t)1 << wanted) - 1;
      result |= ((uint64_t)b & mask) << collected;
      s->bits = (uint64_t)b >> wanted;
      s->bits_left = 8 - wanted;
      collected = n;
    }
  }
  *out = result;
  return ZB_OK;
}

static ZB_UNUSED int zb_read_bytes(zb_stream_t *s, zb_arena_t *arena, size_t n,
                                   zb_bytes_t *out) {
  uint8_t *buf = (uint8_t *)zb_arena_alloc(arena, n ? n : 1);
  if (!buf) {
    return ZB_ERR_ALLOC;
  }
  int err = zb_stream_read_raw(s, buf, n);
  if (err) {
    return err;
  }
  *out = zb_bytes_make(buf, n);
  return ZB_OK;
}

static ZB_UNUSED int zb_read_bytes_full(zb_stream_t *s, zb_arena_t *arena,
                                        zb_bytes_t *out) {
  size_t remaining = s->limit - s->pos;
  return zb_read_bytes(s, arena, remaining, out);
}

static ZB_UNUSED int zb_read_bytes_term(zb_stream_t *s, zb_arena_t *arena,
                                        int term, int include, int consume,
                                        int eos_error, zb_bytes_t *out) {
  zb_buf_t buf;
  zb_buf_init(&buf, arena);
  for (;;) {
    if (s->pos >= s->limit) {
      if (eos_error) {
        return ZB_ERR_EOF;
      }
      *out = zb_buf_to_bytes(&buf);
      return ZB_OK;
    }
    uint8_t b;
    int err = zb_stream_read_raw(s, &b, 1);
    if (err) {
      return err;
    }
    if (b == (uint8_t)term) {
      if (include) {
        err = zb_buf_append_byte(&buf, b);
        if (err) {
          return err;
        }
      }
      if (!consume) {
        s->pos--;
      }
      *out = zb_buf_to_bytes(&buf);
      return ZB_OK;
    }
    err = zb_buf_append_byte(&buf, b);
    if (err) {
      return err;
    }
  }
}

static ZB_UNUSED int zb_read_bytes_term_multi(zb_stream_t *s, zb_arena_t *arena,
                                              const uint8_t *term, size_t unit,
                                              int include, int consume,
                                              int eos_error, zb_bytes_t *out) {
  zb_buf_t buf;
  zb_buf_init(&buf, arena);
  for (;;) {
    if (s->pos + unit > s->limit) {
      if (eos_error) {
        return ZB_ERR_EOF;
      }
      *out = zb_buf_to_bytes(&buf);
      return ZB_OK;
    }
    uint8_t chunk[8];
    if (unit > sizeof(chunk)) {
      return ZB_ERR_UNSUPPORTED;
    }
    int err = zb_stream_read_raw(s, chunk, unit);
    if (err) {
      return err;
    }
    if (memcmp(chunk, term, unit) == 0) {
      if (include) {
        err = zb_buf_append(&buf, chunk, unit);
        if (err) {
          return err;
        }
      }
      if (!consume) {
        s->pos -= unit;
      }
      *out = zb_buf_to_bytes(&buf);
      return ZB_OK;
    }
    err = zb_buf_append(&buf, chunk, unit);
    if (err) {
      return err;
    }
  }
}

static ZB_UNUSED int zb_read_bytes_pad_term(zb_stream_t *s, zb_arena_t *arena,
                                            size_t n, int term, int pad,
                                            int include, zb_bytes_t *out) {
  zb_bytes_t raw;
  int err = zb_read_bytes(s, arena, n, &raw);
  if (err) {
    return err;
  }
  if (term >= 0) {
    ptrdiff_t i = zb_bytes_index(raw, (uint8_t)term);
    if (i >= 0) {
      size_t end = (size_t)i + (include ? 1 : 0);
      *out = zb_bytes_slice(raw, 0, end);
      return ZB_OK;
    }
  }
  if (pad >= 0) {
    *out = zb_bytes_strip_right(raw, pad);
    return ZB_OK;
  }
  *out = raw;
  return ZB_OK;
}

typedef struct zb_writer {
  zb_buf_t buf;
  uint64_t bits;
  int bits_left;
  int bits_le;
} zb_writer_t;

static ZB_UNUSED void zb_writer_init(zb_writer_t *w, zb_arena_t *arena) {
  memset(w, 0, sizeof(*w));
  zb_buf_init(&w->buf, arena);
}

static ZB_UNUSED zb_bytes_t zb_writer_bytes(const zb_writer_t *w) {
  return zb_buf_to_bytes(&w->buf);
}

static ZB_UNUSED size_t zb_writer_pos(const zb_writer_t *w) {
  size_t p = w->buf.len;
  if (w->bits_left > 0) {
    p++;
  }
  return p;
}

static ZB_UNUSED int zb_writer_align_to_byte(zb_writer_t *w) {
  if (w->bits_left == 0) {
    return ZB_OK;
  }
  if (w->bits_le) {
    int err = zb_buf_append_byte(&w->buf, (uint8_t)(w->bits & 0xff));
    w->bits = 0;
    w->bits_left = 0;
    w->bits_le = 0;
    return err;
  } else {
    int shift = 8 - w->bits_left;
    int err = zb_buf_append_byte(&w->buf, (uint8_t)((w->bits << shift) & 0xff));
    w->bits = 0;
    w->bits_left = 0;
    return err;
  }
}

static ZB_UNUSED int zb_writer_byte_aligned(zb_writer_t *w) {
  if (w->bits_left == 0) {
    return ZB_OK;
  }
  return zb_writer_align_to_byte(w);
}

static ZB_UNUSED zb_bytes_t zb_writer_finalize(zb_writer_t *w) {
  (void)zb_writer_align_to_byte(w);
  return zb_buf_to_bytes(&w->buf);
}

static ZB_UNUSED int zb_write_u1(zb_writer_t *w, uint8_t v) {
  int err = zb_writer_byte_aligned(w);
  if (err) {
    return err;
  }
  return zb_buf_append_byte(&w->buf, v);
}
static ZB_UNUSED int zb_write_s1(zb_writer_t *w, int8_t v) {
  int err = zb_writer_byte_aligned(w);
  if (err) {
    return err;
  }
  return zb_buf_append_byte(&w->buf, (uint8_t)v);
}

#define ZB_DEFINE_WRITE_INT(NAME, T, N, BE)                                    \
  static ZB_UNUSED int NAME(zb_writer_t *w, T v) {                             \
    int _err = zb_writer_byte_aligned(w);                                      \
    if (_err) {                                                                \
      return _err;                                                             \
    }                                                                          \
    uint8_t buf[N];                                                            \
    uint64_t u = (uint64_t)v;                                                  \
    if (BE) {                                                                  \
      for (int i = N - 1; i >= 0; i--) {                                       \
        buf[i] = (uint8_t)(u & 0xff);                                          \
        u >>= 8;                                                               \
      }                                                                        \
    } else {                                                                   \
      for (int i = 0; i < N; i++) {                                            \
        buf[i] = (uint8_t)(u & 0xff);                                          \
        u >>= 8;                                                               \
      }                                                                        \
    }                                                                          \
    return zb_buf_append(&w->buf, buf, N);                                     \
  }

ZB_DEFINE_WRITE_INT(zb_write_u2be, uint16_t, 2, 1)
ZB_DEFINE_WRITE_INT(zb_write_u2le, uint16_t, 2, 0)
ZB_DEFINE_WRITE_INT(zb_write_u4be, uint32_t, 4, 1)
ZB_DEFINE_WRITE_INT(zb_write_u4le, uint32_t, 4, 0)
ZB_DEFINE_WRITE_INT(zb_write_u8be, uint64_t, 8, 1)
ZB_DEFINE_WRITE_INT(zb_write_u8le, uint64_t, 8, 0)
ZB_DEFINE_WRITE_INT(zb_write_s2be, int16_t, 2, 1)
ZB_DEFINE_WRITE_INT(zb_write_s2le, int16_t, 2, 0)
ZB_DEFINE_WRITE_INT(zb_write_s4be, int32_t, 4, 1)
ZB_DEFINE_WRITE_INT(zb_write_s4le, int32_t, 4, 0)
ZB_DEFINE_WRITE_INT(zb_write_s8be, int64_t, 8, 1)
ZB_DEFINE_WRITE_INT(zb_write_s8le, int64_t, 8, 0)

#undef ZB_DEFINE_WRITE_INT

static ZB_UNUSED int zb_write_f4be(zb_writer_t *w, float v) {
  uint32_t u;
  memcpy(&u, &v, 4);
  return zb_write_u4be(w, u);
}
static ZB_UNUSED int zb_write_f4le(zb_writer_t *w, float v) {
  uint32_t u;
  memcpy(&u, &v, 4);
  return zb_write_u4le(w, u);
}
static ZB_UNUSED int zb_write_f8be(zb_writer_t *w, double v) {
  uint64_t u;
  memcpy(&u, &v, 8);
  return zb_write_u8be(w, u);
}
static ZB_UNUSED int zb_write_f8le(zb_writer_t *w, double v) {
  uint64_t u;
  memcpy(&u, &v, 8);
  return zb_write_u8le(w, u);
}

static ZB_UNUSED int zb_write_bytes(zb_writer_t *w, zb_bytes_t b) {
  int err = zb_writer_byte_aligned(w);
  if (err) {
    return err;
  }
  return zb_buf_append(&w->buf, b.data, b.len);
}

static ZB_UNUSED int zb_write_bytes_raw(zb_writer_t *w, const void *data,
                                        size_t len) {
  int err = zb_writer_byte_aligned(w);
  if (err) {
    return err;
  }
  return zb_buf_append(&w->buf, (const uint8_t *)data, len);
}

static ZB_UNUSED int zb_write_bytes_limit(zb_writer_t *w, zb_bytes_t b,
                                          size_t size, int term, int pad) {
  int pad_byte = pad < 0 ? 0 : pad;
  size_t to_write = b.len > size ? size : b.len;
  int err = zb_buf_append(&w->buf, b.data, to_write);
  if (err) {
    return err;
  }
  size_t written = to_write;
  if (term >= 0 && written < size) {
    err = zb_buf_append_byte(&w->buf, (uint8_t)term);
    if (err) {
      return err;
    }
    written++;
  }
  while (written < size) {
    err = zb_buf_append_byte(&w->buf, (uint8_t)pad_byte);
    if (err) {
      return err;
    }
    written++;
  }
  return ZB_OK;
}

static ZB_UNUSED int zb_write_bits_be(zb_writer_t *w, int n, uint64_t v) {
  if (n < 0 || n > 64) {
    return ZB_ERR_OVERFLOW;
  }
  if (w->bits_le) {
    int err = zb_writer_align_to_byte(w);
    if (err) {
      return err;
    }
  }
  w->bits_le = 0;
  uint64_t mask = (n == 64) ? ~(uint64_t)0 : ((uint64_t)1 << n) - 1;
  v &= mask;
  while (n > 0) {
    int chunk = n;
    if (w->bits_left + chunk > 56) {
      chunk = 56 - w->bits_left;
    }
    if (chunk <= 0) {
      return ZB_ERR_OVERFLOW;
    }
    uint64_t cv = (v >> (n - chunk)) &
                  ((chunk == 64) ? ~(uint64_t)0 : (((uint64_t)1 << chunk) - 1));
    w->bits = (w->bits << chunk) | cv;
    w->bits_left += chunk;
    n -= chunk;
    while (w->bits_left >= 8) {
      int shift = w->bits_left - 8;
      uint8_t out = (uint8_t)((w->bits >> shift) & 0xff);
      int err = zb_buf_append_byte(&w->buf, out);
      if (err) {
        return err;
      }
      w->bits_left -= 8;
      w->bits &=
          ((w->bits_left == 0) ? 0 : (((uint64_t)1 << w->bits_left) - 1));
    }
  }
  return ZB_OK;
}

static ZB_UNUSED int zb_write_bits_le(zb_writer_t *w, int n, uint64_t v) {
  if (n < 0 || n > 64) {
    return ZB_ERR_OVERFLOW;
  }
  if (!w->bits_le && w->bits_left != 0) {
    int err = zb_writer_align_to_byte(w);
    if (err) {
      return err;
    }
  }
  w->bits_le = 1;
  uint64_t mask = (n == 64) ? ~(uint64_t)0 : ((uint64_t)1 << n) - 1;
  v &= mask;
  w->bits |= v << w->bits_left;
  w->bits_left += n;
  while (w->bits_left >= 8) {
    uint8_t out = (uint8_t)(w->bits & 0xff);
    int err = zb_buf_append_byte(&w->buf, out);
    if (err) {
      return err;
    }
    w->bits >>= 8;
    w->bits_left -= 8;
  }
  return ZB_OK;
}

static ZB_UNUSED zb_bytes_t zb_process_xor(zb_arena_t *arena, zb_bytes_t in,
                                           zb_bytes_t key) {
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, in.len ? in.len : 1);
  if (key.len == 0) {
    memcpy(out, in.data, in.len);
    return zb_bytes_make(out, in.len);
  }
  if (key.len == 1) {
    uint8_t k = key.data[0];
    for (size_t i = 0; i < in.len; i++) {
      out[i] = in.data[i] ^ k;
    }
  } else {
    for (size_t i = 0; i < in.len; i++) {
      out[i] = in.data[i] ^ key.data[i % key.len];
    }
  }
  return zb_bytes_make(out, in.len);
}

static ZB_UNUSED zb_bytes_t zb_process_rotate_left(zb_arena_t *arena,
                                                   zb_bytes_t in, int amount) {
  amount = ((amount % 8) + 8) % 8;
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, in.len ? in.len : 1);
  for (size_t i = 0; i < in.len; i++) {
    uint8_t b = in.data[i];
    out[i] = (uint8_t)((b << amount) | (b >> (8 - amount)));
  }
  return zb_bytes_make(out, in.len);
}

static ZB_UNUSED zb_bytes_t zb_process_rotate_right(zb_arena_t *arena,
                                                    zb_bytes_t in, int amount) {
  return zb_process_rotate_left(arena, in, 8 - (((amount % 8) + 8) % 8));
}

static ZB_UNUSED int zb_process_zlib(zb_arena_t *arena, zb_bytes_t in,
                                     zb_bytes_t *out) {
  z_stream zs;
  memset(&zs, 0, sizeof(zs));
  if (inflateInit(&zs) != Z_OK) {
    return ZB_ERR_FORMAT;
  }
  zs.next_in = (Bytef *)in.data;
  zs.avail_in = (uInt)in.len;
  zb_buf_t buf;
  zb_buf_init(&buf, arena);
  uint8_t chunk[4096];
  int ret;
  do {
    zs.next_out = chunk;
    zs.avail_out = sizeof(chunk);
    ret = inflate(&zs, Z_NO_FLUSH);
    if (ret != Z_OK && ret != Z_STREAM_END) {
      inflateEnd(&zs);
      return ZB_ERR_FORMAT;
    }
    zb_buf_append(&buf, chunk, sizeof(chunk) - zs.avail_out);
  } while (ret != Z_STREAM_END);
  inflateEnd(&zs);
  *out = zb_buf_to_bytes(&buf);
  return ZB_OK;
}

static ZB_UNUSED int zb_unprocess_zlib(zb_arena_t *arena, zb_bytes_t in,
                                       zb_bytes_t *out) {
  z_stream zs;
  memset(&zs, 0, sizeof(zs));
  if (deflateInit(&zs, Z_DEFAULT_COMPRESSION) != Z_OK) {
    return ZB_ERR_FORMAT;
  }
  zs.next_in = (Bytef *)in.data;
  zs.avail_in = (uInt)in.len;
  zb_buf_t buf;
  zb_buf_init(&buf, arena);
  uint8_t chunk[4096];
  int ret;
  do {
    zs.next_out = chunk;
    zs.avail_out = sizeof(chunk);
    ret = deflate(&zs, Z_FINISH);
    if (ret != Z_OK && ret != Z_STREAM_END && ret != Z_BUF_ERROR) {
      deflateEnd(&zs);
      return ZB_ERR_FORMAT;
    }
    zb_buf_append(&buf, chunk, sizeof(chunk) - zs.avail_out);
  } while (ret != Z_STREAM_END);
  deflateEnd(&zs);
  *out = zb_buf_to_bytes(&buf);
  return ZB_OK;
}

static ZB_UNUSED const char *zb_iconv_label(const char *enc) {
  static char buf[64];
  size_t j = 0;
  for (size_t i = 0; enc[i] && j + 1 < sizeof(buf); i++) {
    char c = enc[i];
    if (c == '-' || c == '_') {
      continue;
    }
    if (c >= 'a' && c <= 'z') {
      c = (char)(c - ('a' - 'A'));
    }
    buf[j++] = c;
  }
  buf[j] = '\0';
  if (!strcmp(buf, "UTF8") || !strcmp(buf, "ASCII")) {
    return "UTF-8";
  }
  if (!strcmp(buf, "UTF16")) {
    return "UTF-16";
  }
  if (!strcmp(buf, "UTF16LE")) {
    return "UTF-16LE";
  }
  if (!strcmp(buf, "UTF16BE")) {
    return "UTF-16BE";
  }
  if (!strcmp(buf, "UTF32LE")) {
    return "UTF-32LE";
  }
  if (!strcmp(buf, "UTF32BE")) {
    return "UTF-32BE";
  }
  if (!strcmp(buf, "SJIS") || !strcmp(buf, "SHIFTJIS")) {
    return "SHIFT_JIS";
  }
  if (!strcmp(buf, "EUCJP")) {
    return "EUC-JP";
  }
  if (!strcmp(buf, "EUCKR")) {
    return "EUC-KR";
  }
  if (!strcmp(buf, "GB2312")) {
    return "GB2312";
  }
  if (!strcmp(buf, "GBK")) {
    return "GBK";
  }
  if (!strcmp(buf, "BIG5")) {
    return "BIG5";
  }
  if (!strcmp(buf, "ISO88591") || !strcmp(buf, "LATIN1")) {
    return "ISO-8859-1";
  }
  if (!strcmp(buf, "ISO88592") || !strcmp(buf, "LATIN2")) {
    return "ISO-8859-2";
  }
  if (!strcmp(buf, "WINDOWS1250") || !strcmp(buf, "CP1250")) {
    return "WINDOWS-1250";
  }
  if (!strcmp(buf, "WINDOWS1252") || !strcmp(buf, "CP1252")) {
    return "WINDOWS-1252";
  }
  if (!strcmp(buf, "IBM437") || !strcmp(buf, "CP437")) {
    return "CP437";
  }
  return enc;
}

static ZB_UNUSED int zb_bytes_decode(zb_arena_t *arena, zb_bytes_t in,
                                     const char *from, zb_bytes_t *out) {
  const char *to = "UTF-8";
  const char *fromLabel = zb_iconv_label(from);
  if (!strcmp(fromLabel, "UTF-8")) {
    *out = zb_bytes_dup(arena, in);
    return ZB_OK;
  }
  iconv_t cd = iconv_open(to, fromLabel);
  if (cd == (iconv_t)-1) {
    return ZB_ERR_UNSUPPORTED;
  }
  zb_buf_t buf;
  zb_buf_init(&buf, arena);
  char *inp = (char *)(uintptr_t)in.data;
  size_t inb = in.len;
  char chunk[4096];
  while (inb > 0) {
    char *outp = chunk;
    size_t outb = sizeof(chunk);
    size_t r = iconv(cd, &inp, &inb, &outp, &outb);
    size_t produced = sizeof(chunk) - outb;
    if (produced > 0) {
      zb_buf_append(&buf, (const uint8_t *)chunk, produced);
    }
    if (r == (size_t)-1) {
      int e = errno;
      if (e == E2BIG) {
        continue;
      }
      iconv_close(cd);
      return ZB_ERR_FORMAT;
    }
  }
  iconv_close(cd);
  *out = zb_buf_to_bytes(&buf);
  return ZB_OK;
}

static ZB_UNUSED int zb_bytes_encode(zb_arena_t *arena, zb_bytes_t in,
                                     const char *to, zb_bytes_t *out) {
  const char *from = "UTF-8";
  const char *toLabel = zb_iconv_label(to);
  if (!strcmp(toLabel, "UTF-8")) {
    *out = zb_bytes_dup(arena, in);
    return ZB_OK;
  }
  iconv_t cd = iconv_open(toLabel, from);
  if (cd == (iconv_t)-1) {
    return ZB_ERR_UNSUPPORTED;
  }
  zb_buf_t buf;
  zb_buf_init(&buf, arena);
  char *inp = (char *)(uintptr_t)in.data;
  size_t inb = in.len;
  char chunk[4096];
  while (inb > 0) {
    char *outp = chunk;
    size_t outb = sizeof(chunk);
    size_t r = iconv(cd, &inp, &inb, &outp, &outb);
    size_t produced = sizeof(chunk) - outb;
    if (produced > 0) {
      zb_buf_append(&buf, (const uint8_t *)chunk, produced);
    }
    if (r == (size_t)-1) {
      int e = errno;
      if (e == E2BIG) {
        continue;
      }
      iconv_close(cd);
      return ZB_ERR_FORMAT;
    }
  }
  iconv_close(cd);
  *out = zb_buf_to_bytes(&buf);
  return ZB_OK;
}

static ZB_UNUSED const char *zb_bytes_to_cstr(zb_arena_t *arena, zb_bytes_t b) {
  char *out = (char *)zb_arena_alloc(arena, b.len + 1);
  if (!out) {
    return NULL;
  }
  if (b.len) {
    memcpy(out, b.data, b.len);
  }
  out[b.len] = '\0';
  return out;
}

static ZB_UNUSED zb_bytes_t zb_cstr_to_bytes(const char *s) {
  return zb_bytes_make((const uint8_t *)s, strlen(s));
}

static ZB_UNUSED int64_t zb_str_to_i_strict(zb_arena_t *arena, zb_bytes_t b,
                                            int base, int *err_out) {
  if (b.len == 0) {
    if (err_out && *err_out == 0) {
      *err_out = ZB_ERR_VALIDATION;
    }
    return 0;
  }
  const char *s = zb_bytes_to_cstr(arena, b);
  if (!s) {
    if (err_out && *err_out == 0) {
      *err_out = ZB_ERR_ALLOC;
    }
    return 0;
  }
  if (base == 0) {
    base = 10;
  }
  errno = 0;
  char *endptr = NULL;
  long long v = strtoll(s, &endptr, base);
  if (errno != 0 || endptr == s || endptr == NULL || *endptr != '\0') {
    if (err_out && *err_out == 0) {
      *err_out = ZB_ERR_VALIDATION;
    }
    return 0;
  }
  return (int64_t)v;
}

static ZB_UNUSED zb_bytes_t zb_bytes_decode_to(zb_arena_t *arena, zb_bytes_t in,
                                               zb_bytes_t enc_bytes) {
  zb_bytes_t out = (zb_bytes_t){0};
  const char *enc = zb_bytes_to_cstr(arena, enc_bytes);
  if (!enc) {
    return out;
  }
  (void)zb_bytes_decode(arena, in, enc, &out);
  return out;
}

static ZB_UNUSED size_t zb_utf8_char_count(zb_bytes_t b) {
  size_t n = 0;
  for (size_t i = 0; i < b.len;) {
    uint8_t c = b.data[i];
    size_t expected;
    if (c < 0x80) {
      expected = 1;
    } else if ((c & 0xE0) == 0xC0) {
      expected = 2;
    } else if ((c & 0xF0) == 0xE0) {
      expected = 3;
    } else if ((c & 0xF8) == 0xF0) {
      expected = 4;
    } else {
      expected = 1;
    }
    if (expected > b.len - i) {
      expected = b.len - i;
    }
    if (expected == 0) {
      expected = 1;
    }
    i += expected;
    n++;
  }
  return n;
}

static ZB_UNUSED zb_bytes_t zb_utf8_reverse(zb_arena_t *arena, zb_bytes_t b) {
  if (b.len == 0) {
    return zb_bytes_make(NULL, 0);
  }
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, b.len);
  if (!out) {
    return zb_bytes_make(NULL, 0);
  }
  size_t dst = 0;
  size_t remaining = b.len;
  while (remaining > 0) {
    size_t cp_start = remaining - 1;
    while (cp_start > 0 && (b.data[cp_start] & 0xC0) == 0x80) {
      cp_start--;
    }
    size_t cp_len = remaining - cp_start;
    uint8_t lead = b.data[cp_start];
    size_t expected;
    if (lead < 0x80) {
      expected = 1;
    } else if ((lead & 0xE0) == 0xC0) {
      expected = 2;
    } else if ((lead & 0xF0) == 0xE0) {
      expected = 3;
    } else if ((lead & 0xF8) == 0xF0) {
      expected = 4;
    } else {
      expected = 1;
    }
    if (cp_len != expected) {
      cp_start = remaining - 1;
      cp_len = 1;
    }
    memcpy(out + dst, b.data + cp_start, cp_len);
    dst += cp_len;
    remaining = cp_start;
  }
  return zb_bytes_make(out, b.len);
}

static ZB_UNUSED zb_bytes_t zb_utf8_substring(zb_bytes_t b, size_t start,
                                              size_t end) {
  size_t cp = 0, byte_start = b.len, byte_end = b.len;
  if (start == 0) {
    byte_start = 0;
  }
  for (size_t i = 0; i < b.len;) {
    if (cp == start) {
      byte_start = i;
    }
    if (cp == end) {
      byte_end = i;
      break;
    }
    uint8_t c = b.data[i];
    size_t step;
    if (c < 0x80) {
      step = 1;
    } else if ((c & 0xE0) == 0xC0) {
      step = 2;
    } else if ((c & 0xF0) == 0xE0) {
      step = 3;
    } else if ((c & 0xF8) == 0xF0) {
      step = 4;
    } else {
      step = 1;
    }
    if (step > b.len - i) {
      step = b.len - i;
    }
    if (step == 0) {
      step = 1;
    }
    i += step;
    cp++;
  }
  if (byte_start > b.len) {
    byte_start = b.len;
  }
  if (byte_end < byte_start) {
    byte_end = byte_start;
  }
  return (zb_bytes_t){.data = b.data + byte_start,
                      .len = byte_end - byte_start};
}

static ZB_UNUSED zb_bytes_t zb_int_to_s(zb_arena_t *arena, int64_t v) {
  char buf[24];
  int n = snprintf(buf, sizeof(buf), "%lld", (long long)v);
  if (n < 0) {
    n = 0;
  }
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, (size_t)(n ? n : 1));
  memcpy(out, buf, (size_t)n);
  return zb_bytes_make(out, (size_t)n);
}

static ZB_UNUSED zb_bytes_t zb_int_to_s_base(zb_arena_t *arena, int64_t v,
                                             int base) {
  if (base < 2 || base > 36) {
    base = 10;
  }
  char tmp[68];
  int neg = v < 0;
  uint64_t u = neg ? (uint64_t)(-(v + 1)) + 1 : (uint64_t)v;
  int p = (int)sizeof(tmp);
  if (u == 0) {
    tmp[--p] = '0';
  }
  while (u) {
    int d = (int)(u % (uint64_t)base);
    tmp[--p] = (char)(d < 10 ? '0' + d : 'a' + (d - 10));
    u /= (uint64_t)base;
  }
  if (neg) {
    tmp[--p] = '-';
  }
  int n = (int)sizeof(tmp) - p;
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, (size_t)(n ? n : 1));
  memcpy(out, &tmp[p], (size_t)n);
  return zb_bytes_make(out, (size_t)n);
}

static ZB_UNUSED int64_t zb_div_floor(int64_t a, int64_t b) {
  int64_t q = a / b;
  if ((a % b != 0) && ((a ^ b) < 0)) {
    q--;
  }
  return q;
}

static ZB_UNUSED int64_t zb_mod_floor(int64_t a, int64_t b) {
  int64_t r = a % b;
  if (r != 0 && ((a ^ b) < 0)) {
    r += b;
  }
  return r;
}

#ifdef __cplusplus
} /* extern "C" */
#endif

#endif /* ZB_ZANBATO_H_ */
