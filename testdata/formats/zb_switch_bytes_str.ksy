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
        '"AB"': body_ab
        '"CD"': body_cd
types:
  body_ab:
    seq:
      - id: val
        type: u4
  body_cd:
    seq:
      - id: val
        type: u2
