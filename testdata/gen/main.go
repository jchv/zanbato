// Program gen generates binary test data files for the Zanbato custom test suite.
// Run with: go run ./testdata/gen/
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	outDir := "testdata/src"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	generators := []struct {
		name string
		fn   func() []byte
	}{
		// Expression tests
		{"zb_expr_int_ops.bin", genExprIntOps},
		{"zb_expr_bitwise.bin", genExprBitwise},
		{"zb_expr_cmp_ternary.bin", genExprCmpTernary},
		{"zb_expr_str_methods.bin", genExprStrMethods},
		{"zb_expr_arr_methods.bin", genExprArrMethods},
		{"zb_expr_bytes_ops.bin", genExprBytesOps},

		// Repeat mode tests
		{"zb_repeat_expr_user.bin", genRepeatExprUser},
		{"zb_repeat_eos_basic.bin", genRepeatEosBasic},
		{"zb_repeat_until_complex.bin", genRepeatUntilComplex},
		{"zb_repeat_eos_mixed.bin", genRepeatEosMixed},

		// Switch tests
		{"zb_switch_on_int.bin", genSwitchOnInt},
		{"zb_switch_on_enum.bin", genSwitchOnEnum},
		{"zb_switch_on_calc.bin", genSwitchOnCalc},
		{"zb_switch_default.bin", genSwitchDefault},

		// Instance tests
		{"zb_inst_value_expr.bin", genInstValueExpr},
		{"zb_inst_pos_io.bin", genInstPosIo},
		{"zb_inst_conditional.bin", genInstConditional},
		{"zb_inst_array.bin", genInstArray},

		// Enum tests
		{"zb_enum_in_expr.bin", genEnumInExpr},
		{"zb_enum_negative.bin", genEnumNegative},
		{"zb_enum_multi_field.bin", genEnumMultiField},

		// Conditional tests
		{"zb_if_basic.bin", genIfBasic},
		{"zb_if_nested.bin", genIfNested},

		// Validation tests
		{"zb_valid_eq_pass.bin", genValidEqPass},
		{"zb_valid_range.bin", genValidRange},
		{"zb_valid_anyof.bin", genValidAnyof},
		{"zb_valid_expr.bin", genValidExpr},

		// Process tests
		{"zb_process_xor_val.bin", genProcessXorVal},
		{"zb_process_xor_field.bin", genProcessXorField},

		// Bit field tests
		{"zb_bits_bool_and_int.bin", genBitsBoolAndInt},
		{"zb_bits_large.bin", genBitsLarge},

		// Params tests
		{"zb_params_basic.bin", genParamsBasic},
		{"zb_params_multi.bin", genParamsMulti},

		// String tests
		{"zb_str_encodings.bin", genStrEncodings},
		{"zb_str_term_pad.bin", genStrTermPad},

		// Navigation tests
		{"zb_nav_parent_chain.bin", genNavParentChain},
		{"zb_nav_root_access.bin", genNavRootAccess},

		// Expression coverage tests
		{"zb_expr_logical.bin", genExprLogical},
		{"zb_expr_sizeof.bin", genExprSizeof},
		{"zb_expr_fstring.bin", genExprFstring},
		{"zb_expr_str_to_i.bin", genExprStrToI},
		{"zb_expr_precedence.bin", genExprPrecedence},

		// Coverage gap tests
		{"zb_nav_any_typed.bin", genNavAnyTyped},
		{"zb_process_repeat.bin", genProcessRepeat},

		// Edge case tests
		{"zb_params_expr.bin", genParamsExpr},
		{"zb_params_enum.bin", genParamsEnumCustom},
		{"zb_if_calc.bin", genIfCalc},
		{"zb_switch_int_arith.bin", genSwitchIntArith},
		{"zb_nested_if_expr.bin", genNestedIfExpr},
		{"zb_valid_contents.bin", genValidContents},
		{"zb_inst_pos_repeat.bin", genInstPosRepeat},
		{"zb_switch_bytes_case.bin", genSwitchBytesCase},
		{"zb_repeat_index.bin", genRepeatIndex},
		{"zb_valid_in_enum.bin", genValidInEnum},
	}

	for _, g := range generators {
		data := g.fn()
		path := filepath.Join(outDir, g.name)
		if err := os.WriteFile(path, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (%d bytes)\n", path, len(data))
	}
}

// === Expression Tests ===

// genExprIntOps: a=100(u4le), b=7(u4le), c=-3(s4le)
func genExprIntOps() []byte {
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint32(buf[0:4], 100)
	binary.LittleEndian.PutUint32(buf[4:8], 7)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(0xFFFFFFFD)) // s4le = -3
	return buf
}

// genExprBitwise: val=0xDEADBEEF (u4le)
func genExprBitwise() []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf[0:4], 0xDEADBEEF)
	return buf
}

// genExprCmpTernary: a=10(u2le), b=20(u2le), flag=1(u1)
func genExprCmpTernary() []byte {
	buf := make([]byte, 5)
	binary.LittleEndian.PutUint16(buf[0:2], 10)
	binary.LittleEndian.PutUint16(buf[2:4], 20)
	buf[4] = 1
	return buf
}

// genExprStrMethods: "12345\0" + num_val=42(u4le) + padding to align
func genExprStrMethods() []byte {
	str := []byte("12345\x00")
	buf := make([]byte, len(str)+4)
	copy(buf, str)
	binary.LittleEndian.PutUint32(buf[len(str):], 42)
	return buf
}

// genExprArrMethods: 5 u2le values: [30, 10, 50, 20, 40]
func genExprArrMethods() []byte {
	vals := []uint16{30, 10, 50, 20, 40}
	buf := make([]byte, len(vals)*2)
	for i, v := range vals {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	return buf
}

// genExprBytesOps: 4 bytes [0x41, 0x42, 0x43, 0x44]
func genExprBytesOps() []byte {
	return []byte{0x41, 0x42, 0x43, 0x44}
}

// === Repeat Mode Tests ===

// genRepeatExprUser: count=3(u1) + 3 items of {val: u2le}: [100, 200, 300]
func genRepeatExprUser() []byte {
	buf := make([]byte, 1+3*2)
	buf[0] = 3
	binary.LittleEndian.PutUint16(buf[1:3], 100)
	binary.LittleEndian.PutUint16(buf[3:5], 200)
	binary.LittleEndian.PutUint16(buf[5:7], 300)
	return buf
}

// genRepeatEosBasic: 4 u4le values: [0x11111111, 0x22222222, 0x33333333, 0x44444444]
func genRepeatEosBasic() []byte {
	vals := []uint32{0x11111111, 0x22222222, 0x33333333, 0x44444444}
	buf := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.LittleEndian.PutUint32(buf[i*4:], v)
	}
	return buf
}

// genRepeatUntilComplex: u1 values until >= 0x80: [0x10, 0x20, 0x30, 0x80]
func genRepeatUntilComplex() []byte {
	return []byte{0x10, 0x20, 0x30, 0x80}
}

// genRepeatEosMixed: entries with {tag: u1, data: bytes(2)} until EOF
// 3 entries: {0x01, AA BB}, {0x02, CC DD}, {0x03, EE FF}
func genRepeatEosMixed() []byte {
	return []byte{
		0x01, 0xAA, 0xBB,
		0x02, 0xCC, 0xDD,
		0x03, 0xEE, 0xFF,
	}
}

// === Switch Tests ===

// genSwitchOnInt: 3 TLV records: {tag=1, u2le=0x1234}, {tag=2, u4le=0xDEADBEEF}, {tag=1, u2le=0x5678}
func genSwitchOnInt() []byte {
	buf := make([]byte, 0, 16)
	// Record 1: tag=1, body=type_a (u2le)
	buf = append(buf, 0x01)
	buf = binary.LittleEndian.AppendUint16(buf, 0x1234)
	// Record 2: tag=2, body=type_b (u4le)
	buf = append(buf, 0x02)
	buf = binary.LittleEndian.AppendUint32(buf, 0xDEADBEEF)
	// Record 3: tag=1, body=type_a (u2le)
	buf = append(buf, 0x01)
	buf = binary.LittleEndian.AppendUint16(buf, 0x5678)
	return buf
}

// genSwitchOnEnum: same as switch_on_int but tag values map to enum
func genSwitchOnEnum() []byte {
	return genSwitchOnInt() // Same binary layout
}

// genSwitchOnCalc: a=1(u1), b=2(u1), body for case a+b=3: u2le=0xBEEF
func genSwitchOnCalc() []byte {
	buf := make([]byte, 4)
	buf[0] = 1 // a
	buf[1] = 2 // b
	binary.LittleEndian.PutUint16(buf[2:4], 0xBEEF)
	return buf
}

// genSwitchDefault: tag=99(u1) which doesn't match any case, body: 4 bytes
func genSwitchDefault() []byte {
	return []byte{99, 0xDE, 0xAD, 0xBE, 0xEF}
}

// === Instance Tests ===

// genInstValueExpr: a=10(u4le), b=3(u4le)
func genInstValueExpr() []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:4], 10)
	binary.LittleEndian.PutUint32(buf[4:8], 3)
	return buf
}

// genInstPosIo: header_len=8(u4le), data_offset=8(u4le), then at offset 8: payload "HELLO\0\0\0"
func genInstPosIo() []byte {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint32(buf[0:4], 8) // header_len
	binary.LittleEndian.PutUint32(buf[4:8], 8) // data_offset
	copy(buf[8:], []byte("HELLO\x00\x00\x00")) // payload at offset 8
	return buf
}

// genInstConditional: flag=1(u1), val=42(u4le)
func genInstConditional() []byte {
	buf := make([]byte, 5)
	buf[0] = 1 // flag = true
	binary.LittleEndian.PutUint32(buf[1:5], 42)
	return buf
}

// genInstArray: count=4(u1), then 4 u2le values: [10, 20, 30, 40]
func genInstArray() []byte {
	buf := make([]byte, 1+4*2)
	buf[0] = 4
	binary.LittleEndian.PutUint16(buf[1:3], 10)
	binary.LittleEndian.PutUint16(buf[3:5], 20)
	binary.LittleEndian.PutUint16(buf[5:7], 30)
	binary.LittleEndian.PutUint16(buf[7:9], 40)
	return buf
}

// === Enum Tests ===

// genEnumInExpr: pet=7(u4le) [cat in our enum]
func genEnumInExpr() []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf[0:4], 7)
	return buf
}

// genEnumNegative: val=-1(s1)
func genEnumNegative() []byte {
	return []byte{0xFF} // s1 = -1
}

// genEnumMultiField: 3 x u4le enum values: [4, 7, 12] = [dog, cat, chicken]
func genEnumMultiField() []byte {
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint32(buf[0:4], 4)   // dog
	binary.LittleEndian.PutUint32(buf[4:8], 7)   // cat
	binary.LittleEndian.PutUint32(buf[8:12], 12) // chicken
	return buf
}

// === Conditional Tests ===

// genIfBasic: flag=1(u1), data=0x12345678(u4le)
func genIfBasic() []byte {
	buf := make([]byte, 5)
	buf[0] = 1
	binary.LittleEndian.PutUint32(buf[1:5], 0x12345678)
	return buf
}

// genIfNested: flag_a=1(u1), flag_b=0(u1), val_a=0xAA(u1), val_b_skipped, val_c=0xCC(u1)
func genIfNested() []byte {
	return []byte{
		0x01, // flag_a = true
		0x00, // flag_b = false
		0xAA, // val_a (present because flag_a=true)
		// val_b is skipped because flag_b=false
		0xCC, // val_c (always present)
	}
}

// === Validation Tests ===

// genValidEqPass: magic "KST!" + payload u4le=42
func genValidEqPass() []byte {
	buf := make([]byte, 8)
	copy(buf[0:4], []byte("KST!"))
	binary.LittleEndian.PutUint32(buf[4:8], 42)
	return buf
}

// genValidRange: val=200(u1) - out of range [10, 100]
func genValidRange() []byte {
	return []byte{200}
}

// genValidAnyof: val=5(u1) - not in [1, 2, 3]
func genValidAnyof() []byte {
	return []byte{5}
}

// genValidExpr: a=10(u1), b=5(u1) - b <= a so validation fails (expr: _ > a)
func genValidExpr() []byte {
	return []byte{10, 5}
}

// === Process Tests ===

// genProcessXorVal: 8 bytes XOR'd with 0xAA. Original: "ABCDEFGH" (0x41..0x48)
func genProcessXorVal() []byte {
	orig := []byte("ABCDEFGH")
	for i := range orig {
		orig[i] ^= 0xAA
	}
	return orig
}

// genProcessXorField: key=0x55(u1), then 8 bytes XOR'd with 0x55. Original: "ABCDEFGH"
func genProcessXorField() []byte {
	buf := make([]byte, 9)
	buf[0] = 0x55
	orig := []byte("ABCDEFGH")
	for i, b := range orig {
		buf[1+i] = b ^ 0x55
	}
	return buf
}

// === Bit Field Tests ===

// genBitsBoolAndInt: packed bits: flag(b1)=1, val3(b3)=5, val4(b4)=0xA, then byte_aligned(u1)=0xFF
// Bits: 1_101_1010 = 0xDA, then 0xFF
func genBitsBoolAndInt() []byte {
	return []byte{0xDA, 0xFF}
}

// genBitsLarge: b32=0xDEADBEEF, b32=0xCAFEBABE packed in big-endian bit order
// 8 bytes total
func genBitsLarge() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], 0xDEADBEEF)
	binary.BigEndian.PutUint32(buf[4:8], 0xCAFEBABE)
	return buf
}

// === Params Tests ===

// genParamsBasic: len=5(u1), then 5 bytes of data "HELLO"
func genParamsBasic() []byte {
	return append([]byte{5}, []byte("HELLO")...)
}

// genParamsMulti: flag=1(u1), count=3(u1), then 3 u2le values: [10, 20, 30]
func genParamsMulti() []byte {
	buf := make([]byte, 2+3*2)
	buf[0] = 1 // flag
	buf[1] = 3 // count
	binary.LittleEndian.PutUint16(buf[2:4], 10)
	binary.LittleEndian.PutUint16(buf[4:6], 20)
	binary.LittleEndian.PutUint16(buf[6:8], 30)
	return buf
}

// === String Tests ===

// genStrEncodings: UTF-8 "Hi" (2 bytes) + UTF-16LE "Hi" (4 bytes + 2 BOM/null pad = 4 bytes)
// Layout: utf8_str(2 bytes) + utf16_str(4 bytes)
func genStrEncodings() []byte {
	buf := make([]byte, 0, 10)
	// UTF-8: "Hi" = 0x48, 0x69
	buf = append(buf, 0x48, 0x69)
	// UTF-16LE: "Hi" = 0x48, 0x00, 0x69, 0x00
	buf = append(buf, 0x48, 0x00, 0x69, 0x00)
	return buf
}

// genStrTermPad: padded(10 bytes, pad-right 0), then strz "foo\0", then sized(5 bytes)
func genStrTermPad() []byte {
	buf := make([]byte, 0, 24)
	// Padded string "abc" in 10 bytes (right-padded with 0x00)
	padded := make([]byte, 10)
	copy(padded, "abc")
	buf = append(buf, padded...)
	// Null-terminated string "foo\0"
	buf = append(buf, []byte("foo\x00")...)
	// Fixed-size string "world" (5 bytes)
	buf = append(buf, []byte("world")...)
	return buf
}

// === Navigation Tests ===

// genNavParentChain: root.val=42(u4le), then child.grandchild data
// Layout: root_val(u4le) + child_val(u2le) + grandchild_val(u1)
func genNavParentChain() []byte {
	buf := make([]byte, 7)
	binary.LittleEndian.PutUint32(buf[0:4], 42)
	binary.LittleEndian.PutUint16(buf[4:6], 100)
	buf[6] = 7
	return buf
}

// genNavRootAccess: root.multiplier=3(u4le), then nested.base=10(u2le)
func genNavRootAccess() []byte {
	buf := make([]byte, 6)
	binary.LittleEndian.PutUint32(buf[0:4], 3)
	binary.LittleEndian.PutUint16(buf[4:6], 10)
	return buf
}

// === Expression Coverage Tests ===

// genExprLogical: a=10(u1), b=20(u1), c=0(u1)
func genExprLogical() []byte {
	return []byte{10, 20, 0}
}

// genExprSizeof: a=0x11(u1), b=0x2222(u2le), c=0x33(u1), nested.d=0x44444444(u4le)
func genExprSizeof() []byte {
	buf := make([]byte, 8)
	buf[0] = 0x11
	binary.LittleEndian.PutUint16(buf[1:3], 0x2222)
	buf[3] = 0x33
	binary.LittleEndian.PutUint32(buf[4:8], 0x44444444)
	return buf
}

// genExprFstring: "Hi\0" + val=42(u1) + x=7(u1) + y=3(u1)
func genExprFstring() []byte {
	return []byte{'H', 'i', 0x00, 42, 7, 3}
}

// genExprStrToI: "42\0" + pad=0x00
func genExprStrToI() []byte {
	return []byte{'4', '2', 0x00, 0x00}
}

// genExprPrecedence: single dummy byte; the test exercises constant-folded
// instance expressions, not field data.
func genExprPrecedence() []byte {
	return []byte{0x00}
}

// === Coverage Gap Tests ===

// genNavAnyTyped: multiplier=3(u4le), tag=1(u1), val=10(u2le)
func genNavAnyTyped() []byte {
	buf := make([]byte, 7)
	binary.LittleEndian.PutUint32(buf[0:4], 3)
	buf[4] = 1
	binary.LittleEndian.PutUint16(buf[5:7], 10)
	return buf
}

// genProcessRepeat: count=2(u1), 2 entries of 2 bytes XOR'd with 0xAA
// Original: [0x41,0x42], [0x43,0x44]; XOR 0xAA -> [0xEB,0xE8], [0xE9,0xEE]
func genProcessRepeat() []byte {
	return []byte{0x02, 0xEB, 0xE8, 0xE9, 0xEE}
}

// === Edge Case Tests ===

// genParamsExpr: len_field=3(u1), then "ABC"
func genParamsExpr() []byte {
	return []byte{0x03, 0x41, 0x42, 0x43}
}

// genParamsEnumCustom: mode=1(u1), val=0x1234(u2le)
func genParamsEnumCustom() []byte {
	buf := make([]byte, 3)
	buf[0] = 1
	binary.LittleEndian.PutUint16(buf[1:3], 0x1234)
	return buf
}

// genIfCalc: has_extra=1(u1), extra_val=100(u2le), main_val=200(u2le), tag=0xFF(u1)
func genIfCalc() []byte {
	buf := make([]byte, 6)
	buf[0] = 1
	binary.LittleEndian.PutUint16(buf[1:3], 100)
	binary.LittleEndian.PutUint16(buf[3:5], 200)
	buf[5] = 0xFF
	return buf
}

// genSwitchIntArith: tag=1(u1), value=42(u2le)
func genSwitchIntArith() []byte {
	buf := make([]byte, 3)
	buf[0] = 1
	binary.LittleEndian.PutUint16(buf[1:3], 42)
	return buf
}

// genNestedIfExpr: flag=1(u1), x=5(u1), y=10(u1), extra=0xFF(u1)
func genNestedIfExpr() []byte {
	return []byte{0x01, 0x05, 0x0A, 0xFF}
}

// genValidContents: PNG signature [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A]
func genValidContents() []byte {
	return []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A}
}

// genInstPosRepeat: count=3(u4le), 4 bytes padding, then 3 u2le values [100, 200, 300]
func genInstPosRepeat() []byte {
	buf := make([]byte, 14)
	binary.LittleEndian.PutUint32(buf[0:4], 3)
	// 4 bytes padding at offset 4-7
	binary.LittleEndian.PutUint16(buf[8:10], 100)
	binary.LittleEndian.PutUint16(buf[10:12], 200)
	binary.LittleEndian.PutUint16(buf[12:14], 300)
	return buf
}

// genSwitchBytesCase: magic="AB", val=0x1234(u4le)
func genSwitchBytesCase() []byte {
	buf := make([]byte, 6)
	buf[0] = 0x41 // 'A'
	buf[1] = 0x42 // 'B'
	binary.LittleEndian.PutUint32(buf[2:6], 0x1234)
	return buf
}

// genRepeatIndex: 5 u1 values [0x41, 0x42, 0x43, 0x44, 0x45]
func genRepeatIndex() []byte {
	return []byte{0x41, 0x42, 0x43, 0x44, 0x45}
}

// genValidInEnum: val=99(u1) - not a valid enum member
func genValidInEnum() []byte {
	return []byte{99}
}
