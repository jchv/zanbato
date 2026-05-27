#include "zanbato.h"
#include "zip.h"

#include <stdio.h>
#include <stdlib.h>

#define ZIP_SECTION_LOCAL_FILE 0x0403

int main(int argc, char **argv) {
  if (argc != 2) {
    fprintf(stderr, "Usage: %s <path/to/archive.zip>\n", argv[0]);
    return 2;
  }

  zb_stream_t stream;
  int err = zb_stream_open(&stream, argv[1]);
  if (err) {
    fprintf(stderr, "open %s: %d\n", argv[1], err);
    return 1;
  }

  zb_arena_t arena;
  zb_arena_init(&arena);

  zip_t root = {0};
  err = zip_read(&root, &stream, &arena, NULL, NULL);
  if (err) {
    fprintf(stderr, "parse %s: %d\n", argv[1], err);
    zb_arena_destroy(&arena);
    zb_stream_close(&stream);
    return 1;
  }

  for (size_t i = 0; i < root.sections.len; i++) {
    zip_pk_section_t *sec = root.sections.data[i];
    if (!sec || sec->section_type != ZIP_SECTION_LOCAL_FILE) {
      continue;
    }
    zip_local_file_t *lf = (zip_local_file_t *)sec->body;
    if (!lf || !lf->header) {
      continue;
    }
    zb_bytes_t name = lf->header->file_name;
    fwrite(name.data, 1, name.len, stdout);
    fputc('\n', stdout);
  }

  zb_arena_destroy(&arena);
  zb_stream_close(&stream);
  return 0;
}
