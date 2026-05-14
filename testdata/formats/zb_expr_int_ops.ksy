meta:
  id: zb_expr_int_ops
  endian: le
seq:
  - id: a
    type: u4
  - id: b
    type: u4
  - id: c
    type: s4
instances:
  add_ab:
    value: a + b
  sub_ab:
    value: a - b
  mul_bc:
    value: b * c
  div_ab:
    value: a / b
  mod_ab:
    value: a % b
