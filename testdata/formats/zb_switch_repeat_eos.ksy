meta:
  id: zb_switch_repeat_eos
  endian: le
seq:
  - id: entries
    type: entry
    repeat: eos
types:
  entry:
    seq:
      - id: tag
        type: u1
      - id: body
        type:
          switch-on: tag
          cases:
            1: small_body
            2: large_body
    types:
      small_body:
        seq:
          - id: val
            type: u2
      large_body:
        seq:
          - id: val
            type: u4
