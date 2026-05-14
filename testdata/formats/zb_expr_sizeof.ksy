meta:
  id: zb_expr_sizeof
  endian: le
seq:
  - id: a
    type: u1
  - id: b
    type: u2
  - id: c
    type: u1
  - id: nested
    type: inner
types:
  inner:
    seq:
      - id: d
        type: u4
instances:
  sizeof_a:
    value: a._sizeof
  sizeof_b:
    value: b._sizeof
  sizeof_nested:
    value: nested._sizeof
