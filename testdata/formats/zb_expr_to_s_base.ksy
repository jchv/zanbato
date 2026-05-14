meta:
  id: zb_expr_to_s_base
seq:
  - id: val
    type: u1
  - id: val2
    type: u1
instances:
  decimal:
    value: val.to_s
  hex:
    value: val.to_s(16)
  octal:
    value: val2.to_s(8)
