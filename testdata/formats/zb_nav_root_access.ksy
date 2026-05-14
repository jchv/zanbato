meta:
  id: zb_nav_root_access
  endian: le
seq:
  - id: multiplier
    type: u4
  - id: nested
    type: nested_type
types:
  nested_type:
    seq:
      - id: base
        type: u2
    instances:
      product:
        value: base * _root.multiplier
