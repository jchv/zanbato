meta:
  id: zb_neg_in_enum_false
seq:
  - id: foo
    type: u1
    enum: animals
    valid:
      in-enum: false
enums:
  animals:
    1: cat
    2: dog
