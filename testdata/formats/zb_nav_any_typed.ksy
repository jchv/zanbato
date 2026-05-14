meta:
  id: zb_nav_any_typed
  endian: le
seq:
  - id: multiplier
    type: u4
  - id: entry
    type: tagged_entry
types:
  tagged_entry:
    seq:
      - id: tag
        type: u1
      - id: body
        type:
          switch-on: tag
          cases:
            1: body_a
            2: body_b
    types:
      body_a:
        seq:
          - id: val
            type: u2
        instances:
          scaled:
            value: val * _root.multiplier
      body_b:
        seq:
          - id: val
            type: u4
