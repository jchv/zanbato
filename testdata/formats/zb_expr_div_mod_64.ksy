meta:
  id: zb_expr_div_mod_64
  endian: le
seq:
  - id: val_u8
    type: u8
  - id: val_s8
    type: s8
instances:
  div_u8:
    value: val_u8 / 100
  div_s8:
    value: val_s8 / 100
  mod_u8:
    value: val_u8 % 100
  mod_s8:
    value: val_s8 % 100
  invert_u8:
    value: ~val_u8
  invert_s8:
    value: ~val_s8
