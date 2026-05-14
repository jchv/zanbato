meta:
  id: zb_debug_primitives
  endian: le
  ks-debug: true
seq:
  - id: header
    type: u2
  - id: items
    type: u2
    repeat: expr
    repeat-expr: 4
