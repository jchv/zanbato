meta:
  id: zb_expr_float_literals
seq:
  - id: dummy
    type: u1
instances:
  digits_only_exp:
    value: 4e2          # 400.0
  digits_only_exp_caps:
    value: 4E2
  digits_only_exp_signed:
    value: 4e-2         # 0.04
  digits_only_exp_pos:
    value: 4e+2
  fixed_normal:
    value: 4.2
  fixed_leading_dot:
    value: .42          # 0.42
  fixed_trailing_dot:
    value: 42.          # 42.0
  fixed_leading_dot_exp:
    value: .5e2         # 50.0
  fixed_trailing_dot_exp:
    value: 4.E2         # 400.0
  fixed_full_exp_neg:
    value: 4.2e-1       # 0.42
  negative_leading_dot:
    value: -.5          # -0.5
