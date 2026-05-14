meta:
  id: zb_str_term_pad
  encoding: ASCII
seq:
  - id: padded
    type: str
    size: 10
    pad-right: 0
  - id: terminated
    type: strz
  - id: sized
    type: str
    size: 5
