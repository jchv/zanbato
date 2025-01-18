package zanbato

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jchv/zanbato/kaitai/emitter/golang"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	TestCompiledPath = ".test/compiled"
)

func TestCodeGeneration(t *testing.T) {
	t.Parallel()

	var m sync.RWMutex

	matches, err := filepath.Glob("internal/third_party/kaitai_struct_tests/formats/*.ksy")
	require.NoError(t, err)

	knownFailing := map[string]struct{}{
		"cast_nested.ksy":                                {},
		"cast_to_imported.ksy":                           {},
		"cast_to_imported2.ksy":                          {},
		"cast_to_top.ksy":                                {},
		"combine_bytes.ksy":                              {},
		"combine_str.ksy":                                {},
		"debug_array_user_current_excluded.ksy":          {},
		"expr_1.ksy":                                     {},
		"expr_2.ksy":                                     {},
		"expr_bytes_non_literal.ksy":                     {},
		"expr_fstring_0.ksy":                             {},
		"expr_int_div.ksy":                               {},
		"expr_io_eof.ksy":                                {},
		"expr_io_pos.ksy":                                {},
		"expr_io_ternary.ksy":                            {},
		"expr_mod.ksy":                                   {},
		"expr_ops_parens.ksy":                            {},
		"expr_sizeof_type_0.ksy":                         {},
		"expr_sizeof_type_1.ksy":                         {},
		"expr_str_encodings.ksy":                         {},
		"expr_str_ops.ksy":                               {},
		"expr_to_i_trailing.ksy":                         {},
		"float_to_i.ksy":                                 {},
		"imports_abs.ksy":                                {},
		"imports_abs_abs.ksy":                            {},
		"imports_abs_rel.ksy":                            {},
		"imports_cast_to_imported.ksy":                   {},
		"imports_cast_to_imported2.ksy":                  {},
		"imports_params_def_array_usertype_imported.ksy": {},
		"index_sizes.ksy":                                {},
		"index_to_param_eos.ksy":                         {},
		"index_to_param_expr.ksy":                        {},
		"index_to_param_until.ksy":                       {},
		"io_local_var.ksy":                               {},
		"nav_parent.ksy":                                 {},
		"nav_parent2.ksy":                                {},
		"nav_parent3.ksy":                                {},
		"nav_parent_false.ksy":                           {},
		"nav_parent_override.ksy":                        {},
		"nav_parent_recursive.ksy":                       {},
		"nav_parent_switch.ksy":                          {},
		"nav_parent_switch_cast.ksy":                     {},
		"nested_same_name.ksy":                           {},
		"nested_same_name2.ksy":                          {},
		"opaque_external_type.ksy":                       {},
		"opaque_external_type_02_child.ksy":              {},
		"opaque_external_type_02_parent.ksy":             {},
		"opaque_with_param.ksy":                          {},
		"params_call.ksy":                                {},
		"params_def.ksy":                                 {},
		"params_def_array_usertype_imported.ksy":         {},
		"params_pass_array_int.ksy":                      {},
		"params_pass_array_str.ksy":                      {},
		"params_pass_array_struct.ksy":                   {},
		"params_pass_array_usertype.ksy":                 {},
		"params_pass_bool.ksy":                           {},
		"params_pass_struct.ksy":                         {},
		"position_in_seq.ksy":                            {},
		"process_coerce_bytes.ksy":                       {},
		"process_coerce_switch.ksy":                      {},
		"process_coerce_usertype1.ksy":                   {},
		"process_coerce_usertype2.ksy":                   {},
		"process_custom.ksy":                             {},
		"process_repeat_bytes.ksy":                       {},
		"process_repeat_usertype.ksy":                    {},
		"process_rotate.ksy":                             {},
		"process_to_user.ksy":                            {},
		"process_xor4_const.ksy":                         {},
		"process_xor4_value.ksy":                         {},
		"process_xor_const.ksy":                          {},
		"process_xor_value.ksy":                          {},
		"repeat_until_s4.ksy":                            {},
		"str_encodings_escaping_to_s.ksy":                {},
		"str_literals.ksy":                               {},
		"switch_bytearray.ksy":                           {},
		"switch_cast.ksy":                                {},
		"switch_else_only.ksy":                           {},
		"switch_integers2.ksy":                           {},
		"switch_manual_enum_invalid_else.ksy":            {},
		"switch_manual_int_else.ksy":                     {},
		"switch_manual_int_size_else.ksy":                {},
		"switch_manual_int_size_eos.ksy":                 {},
		"switch_manual_str_else.ksy":                     {},
		"term_bytes.ksy":                                 {},
		"term_u1_val.ksy":                                {},
		"type_int_unary_op.ksy":                          {},
		"type_ternary.ksy":                               {},
		"type_ternary_2nd_falsy.ksy":                     {},
		"type_ternary_opaque.ksy":                        {},
		"valid_fail_contents_inst.ksy":                   {},
		"valid_fail_inst.ksy":                            {},
		"valid_fail_repeat_inst.ksy":                     {},
		"valid_switch.ksy":                               {},
	}
	newlyFailing := []string{}
	newlyPassing := []string{}
	t.Cleanup(func() {
		m.Lock()
		if len(knownFailing) > 0 {
			t.Errorf("Unknown tests in known failing list: %v", knownFailing)
		}
		if len(newlyFailing) > 0 {
			t.Errorf("New tests failing: %#v", newlyFailing)
		}
		if len(newlyPassing) > 0 {
			t.Errorf("New tests passing: %#v", newlyPassing)
		}
		m.Unlock()
	})

	for _, match := range matches {
		match := match
		name := filepath.Base(match)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tf := func() {
				resolver := resolve.NewOSResolver()
				emitter := golang.NewEmitter("test_formats", resolver)
				basename, struc, err := resolver.Resolve("", match)
				if err != nil {
					log.Fatalf("Error resolving root struct: %v", err)
				}
				os.MkdirAll(TestCompiledPath, os.ModeDir|0o755)
				artifacts := emitter.Emit(basename, struc)
				for _, artifact := range artifacts {
					outname := filepath.Join(TestCompiledPath, artifact.Filename)
					file, err := os.Create(outname)
					if err != nil {
						log.Fatalf("Error creating %s: %v", outname, err)
					}
					_, err = file.Write(artifact.Body)
					if err != nil {
						log.Fatalf("Error writing %s: %v", outname, err)
					}
				}
			}

			m.RLock()
			_, ok := knownFailing[name]
			m.RUnlock()
			if ok {
				if !assert.Panics(t, tf) {
					m.Lock()
					newlyPassing = append(newlyPassing, name)
					m.Unlock()
				}
				m.Lock()
				delete(knownFailing, name)
				m.Unlock()
			} else if !assert.NotPanics(t, tf) {
				m.Lock()
				newlyFailing = append(newlyFailing, name)
				m.Unlock()
			}
		})
	}
}
