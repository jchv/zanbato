meta:
  id: zb_if_calc
  endian: le
seq:
  - id: has_extra
    type: u1
  - id: extra_val
    type: u2
    if: has_extra != 0
  - id: main_val
    type: u2
  - id: tag
    type: u1
instances:
  combined:
    value: 'has_extra != 0 ? extra_val + main_val : main_val'
