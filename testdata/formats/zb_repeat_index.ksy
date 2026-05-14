meta:
  id: zb_repeat_index
seq:
  - id: items
    type: u1
    repeat: until
    repeat-until: _index >= 3
