meta:
  id: zb_expr_logical
seq:
  - id: a
    type: u1
  - id: b
    type: u1
  - id: c
    type: u1
instances:
  and_true:
    value: (a > 5) and (b > 10)
  and_false:
    value: (a > 5) and (c > 10)
  or_true:
    value: (a > 50) or (b > 10)
  or_false:
    value: (a > 50) or (c > 10)
  not_true:
    value: not (c > 10)
