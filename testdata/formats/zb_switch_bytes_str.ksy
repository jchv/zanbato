meta:
  id: zb_switch_bytes_str
  endian: le
seq:
  - id: magic
    size: 2
  - id: body
    type:
      switch-on: magic
      cases:
        '[0x41, 0x42]': body_ab
        '[0x43, 0x44]': body_cd
types:
  body_ab:
    seq:
      - id: val
        type: u4
  body_cd:
    seq:
      - id: val
        type: u2
