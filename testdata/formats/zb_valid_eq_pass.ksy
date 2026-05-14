meta:
  id: zb_valid_eq_pass
  endian: le
seq:
  - id: magic
    size: 4
    valid:
      eq: '[0x4B, 0x53, 0x54, 0x21]'
  - id: payload
    type: u4
