meta:
  id: zb_switch_default
  endian: le
seq:
  - id: tag
    type: u1
  - id: body
    type:
      switch-on: tag
      cases:
        1: known_type
        _: fallback_type
types:
  known_type:
    seq:
      - id: val
        type: u2
  fallback_type:
    seq:
      - id: data
        size: 4
