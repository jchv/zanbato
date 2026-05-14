meta:
  id: zb_debug_repeat_until
  ks-debug: true
seq:
  - id: items
    type: u1
    repeat: until
    repeat-until: _ >= 0x80
