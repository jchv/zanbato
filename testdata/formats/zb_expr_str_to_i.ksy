meta:
  id: zb_expr_str_to_i
  encoding: ASCII
seq:
  - id: dec_str
    type: strz
  - id: pad
    type: u1
instances:
  parsed_dec:
    value: dec_str.to_i(10)
