meta:
  id: zb_enum_multi_field
  endian: le
seq:
  - id: pet_1
    type: u4
    enum: animal
  - id: pet_2
    type: u4
    enum: animal
  - id: pet_3
    type: u4
    enum: animal
enums:
  animal:
    4: dog
    7: cat
    12: chicken
