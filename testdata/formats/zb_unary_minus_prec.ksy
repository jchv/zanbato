meta:
  id: zb_unary_minus_prec
  endian: le
seq:
  - id: val
    type: u4
instances:
  neg_to_s_len:
    value: -val.to_s.length
