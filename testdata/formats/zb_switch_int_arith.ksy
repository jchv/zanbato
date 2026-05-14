meta:
  id: zb_switch_int_arith
  endian: le
seq:
  - id: tag
    type: u1
  - id: value
    type:
      switch-on: tag
      cases:
        1: u2
        2: u4
instances:
  doubled:
    value: value * 2
