#include "zanbato.h"
#include <string.h>

zb_bytes_t my_custom_fx_decode(zb_arena_t *arena, zb_bytes_t in, int key,
                               int flag, zb_bytes_t some_bytes) {
  (void)some_bytes;
  int k = flag ? key : -key;
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, in.len ? in.len : 1);
  for (size_t i = 0; i < in.len; i++) {
    out[i] = (uint8_t)(in.data[i] + k);
  }
  return zb_bytes_make(out, in.len);
}

zb_bytes_t my_custom_fx_encode(zb_arena_t *arena, zb_bytes_t in, int key,
                               int flag, zb_bytes_t some_bytes) {
  (void)some_bytes;
  int k = flag ? key : -key;
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, in.len ? in.len : 1);
  for (size_t i = 0; i < in.len; i++) {
    out[i] = (uint8_t)(in.data[i] - k);
  }
  return zb_bytes_make(out, in.len);
}

zb_bytes_t custom_fx_decode(zb_arena_t *arena, zb_bytes_t in, int key) {
  (void)key;
  size_t n = in.len + 2;
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, n);
  out[0] = '_';
  if (in.len) {
    memcpy(out + 1, in.data, in.len);
  }
  out[n - 1] = '_';
  return zb_bytes_make(out, n);
}

zb_bytes_t custom_fx_encode(zb_arena_t *arena, zb_bytes_t in, int key) {
  (void)key;
  if (in.len < 2) {
    return zb_bytes_make(NULL, 0);
  }
  size_t n = in.len - 2;
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, n ? n : 1);
  if (n) {
    memcpy(out, in.data + 1, n);
  }
  return zb_bytes_make(out, n);
}

zb_bytes_t custom_fx_no_args_decode(zb_arena_t *arena, zb_bytes_t in) {
  size_t n = in.len + 2;
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, n);
  out[0] = '_';
  if (in.len) {
    memcpy(out + 1, in.data, in.len);
  }
  out[n - 1] = '_';
  return zb_bytes_make(out, n);
}

zb_bytes_t custom_fx_no_args_encode(zb_arena_t *arena, zb_bytes_t in) {
  if (in.len < 2) {
    return zb_bytes_make(NULL, 0);
  }
  size_t n = in.len - 2;
  uint8_t *out = (uint8_t *)zb_arena_alloc(arena, n ? n : 1);
  if (n) {
    memcpy(out, in.data + 1, n);
  }
  return zb_bytes_make(out, n);
}
