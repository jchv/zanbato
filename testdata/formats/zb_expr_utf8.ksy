meta:
  id: zb_expr_utf8
  endian: le
seq:
  - id: str_utf8_bytes
    type: strz
    encoding: UTF-8
  - id: str_utf16le_bytes
    size: 12
instances:
  str_len:
    value: str_utf8_bytes.length
  str_sub:
    value: str_utf8_bytes.substring(1, 3)
  str_utf16le:
    value: str_utf16le_bytes.to_s("UTF-16LE")
