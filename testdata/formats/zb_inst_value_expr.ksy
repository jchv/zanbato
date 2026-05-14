meta:
  id: zb_inst_value_expr
  endian: le
seq:
  - id: a
    type: u4
  - id: b
    type: u4
instances:
  sum:
    value: a + b
  product:
    value: a * b
  is_bigger:
    value: a > b
