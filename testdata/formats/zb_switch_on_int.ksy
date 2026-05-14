meta:
  id: zb_switch_on_int
  endian: le
seq:
  - id: records
    type: record
    repeat: eos
types:
  record:
    seq:
      - id: tag
        type: u1
      - id: body
        type:
          switch-on: tag
          cases:
            1: type_a
            2: type_b
    types:
      type_a:
        seq:
          - id: val
            type: u2
      type_b:
        seq:
          - id: val
            type: u4
