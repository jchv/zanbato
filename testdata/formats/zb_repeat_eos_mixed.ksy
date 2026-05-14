meta:
  id: zb_repeat_eos_mixed
seq:
  - id: entries
    type: entry
    repeat: eos
types:
  entry:
    seq:
      - id: tag
        type: u1
      - id: data
        size: 2
