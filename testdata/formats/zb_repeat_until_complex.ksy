meta:
  id: zb_repeat_until_complex
seq:
  - id: items
    type: u1
    repeat: until
    repeat-until: _ >= 0x80
