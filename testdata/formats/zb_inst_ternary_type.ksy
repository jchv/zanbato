meta:
  id: zb_inst_ternary_type
  endian: le
seq:
  - id: a
    type: u2
  - id: b
    type: u2
  - id: flag
    type: u1
instances:
  chosen:
    value: 'flag != 0 ? a : b'
  double_chosen:
    value: 'flag != 0 ? a * 2 : b * 2'
