meta:
  id: zb_nav_parent_chain
  endian: le
seq:
  - id: root_val
    type: u4
  - id: child
    type: child_type
types:
  child_type:
    seq:
      - id: child_val
        type: u2
      - id: grandchild
        type: grandchild_type
    types:
      grandchild_type:
        seq:
          - id: grandchild_val
            type: u1
        instances:
          root_val_via_parent:
            value: _parent._parent.root_val
