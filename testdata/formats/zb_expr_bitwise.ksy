meta:
  id: zb_expr_bitwise
  endian: le
seq:
  - id: val
    type: u4
instances:
  shl_4:
    value: val << 4
  shr_8:
    value: val >> 8
  band:
    value: val & 0xFF
  bor:
    value: val | 0xFF000000
  bxor:
    value: val ^ 0xFFFFFFFF
