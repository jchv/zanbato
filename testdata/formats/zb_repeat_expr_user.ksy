meta:
  id: zb_repeat_expr_user
  endian: le
seq:
  - id: count
    type: u1
  - id: items
    type: item
    repeat: expr
    repeat-expr: count
types:
  item:
    seq:
      - id: val
        type: u2
