meta:
  id: zb_if_inst
  endian: le
seq:
  - id: flag
    type: u1
  - id: val
    type: u4
instances:
  computed:
    value: val + 8
    if: flag != 0
  always:
    value: val
