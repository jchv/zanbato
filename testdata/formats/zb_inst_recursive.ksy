meta:
  id: zb_inst_recursive
  endian: le
seq:
  - id: a
    type: u4
  - id: b
    type: u4
instances:
  sum:
    value: a + b
  double_sum:
    value: sum * 2
  is_big:
    value: double_sum > 20
