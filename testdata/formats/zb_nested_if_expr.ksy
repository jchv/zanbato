meta:
  id: zb_nested_if_expr
seq:
  - id: flag
    type: u1
  - id: inner
    type: inner_type(flag)
  - id: extra
    type: u1
types:
  inner_type:
    params:
      - id: include_y
        type: u1
    seq:
      - id: x
        type: u1
      - id: y
        type: u1
        if: include_y != 0
    instances:
      sum:
        value: 'include_y != 0 ? x + y : x'
