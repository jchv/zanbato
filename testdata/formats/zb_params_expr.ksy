meta:
  id: zb_params_expr
  endian: le
seq:
  - id: len_field
    type: u1
  - id: body
    type: sized_block(len_field * 2 - 3)
types:
  sized_block:
    params:
      - id: block_size
        type: u4
    seq:
      - id: data
        size: block_size
