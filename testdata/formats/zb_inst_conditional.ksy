meta:
  id: zb_inst_conditional
  endian: le
seq:
  - id: flag
    type: u1
  - id: val
    type: u4
instances:
  double_val:
    value: val * 2
    if: flag != 0
