meta:
  id: zb_expr_str_methods
  endian: le
  encoding: ASCII
seq:
  - id: str_val
    type: strz
  - id: num_val
    type: u4
instances:
  str_len:
    value: str_val.length
  str_rev:
    value: str_val.reverse
  str_sub:
    value: str_val.substring(1, 4)
  num_str:
    value: num_val.to_s
