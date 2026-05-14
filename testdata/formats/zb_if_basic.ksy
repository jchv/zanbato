meta:
  id: zb_if_basic
  endian: le
seq:
  - id: flag
    type: u1
  - id: data
    type: u4
    if: flag != 0
