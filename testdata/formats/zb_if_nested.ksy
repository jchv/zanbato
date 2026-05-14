meta:
  id: zb_if_nested
seq:
  - id: flag_a
    type: u1
  - id: flag_b
    type: u1
  - id: val_a
    type: u1
    if: flag_a != 0
  - id: val_b
    type: u1
    if: flag_b != 0
  - id: val_c
    type: u1
