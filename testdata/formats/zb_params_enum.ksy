meta:
  id: zb_params_enum
  endian: le
enums:
  mode_type:
    1: short
    2: long
seq:
  - id: mode
    type: u1
    enum: mode_type
  - id: body
    type: data_block(mode)
types:
  data_block:
    params:
      - id: fmt
        type: u1
        enum: mode_type
    seq:
      - id: content
        type:
          switch-on: fmt
          cases:
            'mode_type::short': short_data
            'mode_type::long': long_data
    types:
      short_data:
        seq:
          - id: val
            type: u2
      long_data:
        seq:
          - id: val
            type: u4
