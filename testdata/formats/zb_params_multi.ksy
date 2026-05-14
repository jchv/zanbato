meta:
  id: zb_params_multi
  endian: le
seq:
  - id: flag
    type: u1
  - id: count
    type: u1
  - id: body
    type: data_block(flag != 0, count)
types:
  data_block:
    params:
      - id: is_active
        type: bool
      - id: num_items
        type: u4
    seq:
      - id: items
        type: u2
        repeat: expr
        repeat-expr: num_items
        if: is_active
