meta:
  id: zb_expr_precedence
  endian: le
seq:
  - id: dummy
    type: u1
instances:
  add_shl:
    value: 1 + 2 << 3
  or_xor:
    value: 1 | 2 ^ 3
  or_and:
    value: 0xff | 0x80 & 0x10
  and_shl:
    value: 0x10 & 0xff << 4
