meta:
  id: zb_expr_invert
  endian: le
seq:
  - id: val
    type: u4
instances:
  inv_val:
    value: ~val
  inv_lit:
    value: ~7
  inv_plus:
    value: ~7 + 3
  inv_paren:
    value: ~(7 + 3)
