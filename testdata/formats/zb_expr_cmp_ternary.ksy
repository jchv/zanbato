meta:
  id: zb_expr_cmp_ternary
  endian: le
seq:
  - id: a
    type: u2
  - id: b
    type: u2
  - id: flag
    type: u1
instances:
  is_eq:
    value: a == b
  is_ne:
    value: a != b
  is_lt:
    value: a < b
  is_le:
    value: a <= b
  is_gt:
    value: a > b
  is_ge:
    value: a >= b
  tern_result:
    value: 'flag != 0 ? a : b'
