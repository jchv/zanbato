meta:
  id: zb_switch_on_enum
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
        enum: record_type
      - id: body
        type:
          switch-on: tag
          cases:
            'record_type::small': type_a
            'record_type::large': type_b
    enums:
      record_type:
        1: small
        2: large
    types:
      type_a:
        seq:
          - id: val
            type: u2
      type_b:
        seq:
          - id: val
            type: u4
