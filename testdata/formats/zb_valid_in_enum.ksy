meta:
  id: zb_valid_in_enum
seq:
  - id: val
    type: u1
    enum: color
    valid:
      any-of:
        - color::red
        - color::green
        - color::blue
enums:
  color:
    1: red
    2: green
    3: blue
