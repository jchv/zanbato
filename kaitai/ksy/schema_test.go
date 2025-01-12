package ksy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFullSchema(t *testing.T) {
	tests := []struct {
		filename string
		expected TypeSpec
	}{
		{
			"elf.ksy",
			TypeSpec{
				Meta: MetaSpec{
					ID:          "elf",
					Title:       "Executable and Linkable Format",
					Application: MultiString{"SVR4 ABI and up, many *nix systems"},
					License:     "CC0-1.0",
					KSVersion:   "0.8",
				},
				Seq: AttributesSpec{
					AttributeSpec{
						ID:       "magic",
						Doc:      "File identification, must be 0x7f + \"ELF\".",
						Contents: ByteSpec{0x7f, 'E', 'L', 'F'},
						Size:     "4",
					},
					AttributeSpec{
						ID:   "bits",
						Doc:  "File class: designates target machine word size (32 or 64\nbits). The size of many integer fields in this format will\ndepend on this setting.\n",
						Type: AttrTypeSpec{Value: "u1"},
						Enum: "bits",
					},
					AttributeSpec{
						ID:   "endian",
						Doc:  "Endianness used for all integers.",
						Type: AttrTypeSpec{Value: "u1"},
						Enum: "endian",
					},
					AttributeSpec{
						ID:   "ei_version",
						Doc:  "ELF header version.",
						Type: AttrTypeSpec{Value: "u1"},
					},
					AttributeSpec{
						ID:   "abi",
						Doc:  "Specifies which OS- and ABI-related extensions will be used\nin this ELF file.\n",
						Type: AttrTypeSpec{Value: "u1"},
						Enum: "os_abi",
					},
					AttributeSpec{
						ID:   "abi_version",
						Doc:  "Version of ABI targeted by this ELF file. Interpretation\ndepends on `abi` attribute.\n",
						Type: AttrTypeSpec{Value: "u1"},
					},
					AttributeSpec{
						ID:   "pad",
						Size: "7",
					},
					AttributeSpec{
						ID:   "header",
						Type: AttrTypeSpec{Value: "endian_elf"},
					},
				},
				Types: TypesSpec{
					TypeSpec{
						Meta: MetaSpec{
							ID: "phdr_type_flags",
						},
						Params: ParamsSpec{
							ParamSpec{
								ID:   "value",
								Type: "u4",
							},
						},
						Instances: InstancesSpec{
							Instances: []InstanceSpecItem{
								{
									Key: "read",
									Value: InstanceSpec{
										Value: "value & 0x04 != 0",
									},
								},
								{
									Key: "write",
									Value: InstanceSpec{
										Value: "value & 0x02 != 0",
									},
								},
								{
									Key: "execute",
									Value: InstanceSpec{
										Value: "value & 0x01 != 0",
									},
								},
								{
									Key: "mask_proc",
									Value: InstanceSpec{
										Value: "value & 0xf0000000 != 0",
									},
								},
							},
						},
					},
					TypeSpec{
						Meta: MetaSpec{
							ID: "section_header_flags",
						},
						Params: ParamsSpec{
							ParamSpec{
								ID:   "value",
								Type: "u4",
							},
						},
						Instances: InstancesSpec{
							Instances: []InstanceSpecItem{
								{
									Key: "write",
									Value: InstanceSpec{
										Doc:   "writable",
										Value: "value & 0x01 != 0",
									},
								},
								{
									Key: "alloc",
									Value: InstanceSpec{
										Doc:   "occupies memory during execution",
										Value: "value & 0x02 != 0",
									},
								},
								{
									Key: "exec_instr",
									Value: InstanceSpec{
										Doc:   "executable",
										Value: "value & 0x04 != 0",
									},
								},
								{
									Key: "merge",
									Value: InstanceSpec{
										Doc:   "might be merged",
										Value: "value & 0x10 != 0",
									},
								},
								{
									Key: "strings",
									Value: InstanceSpec{
										Doc:   "contains nul-terminated strings",
										Value: "value & 0x20 != 0",
									},
								},
								{
									Key: "info_link",
									Value: InstanceSpec{
										Doc:   "'sh_info' contains SHT index",
										Value: "value & 0x40 != 0",
									},
								},
								{
									Key: "link_order",
									Value: InstanceSpec{
										Doc:   "preserve order after combining",
										Value: "value & 0x80 != 0",
									},
								},
								{
									Key: "os_non_conforming",
									Value: InstanceSpec{
										Doc:   "non-standard OS specific handling required",
										Value: "value & 0x100 != 0",
									},
								},
								{
									Key: "group",
									Value: InstanceSpec{
										Doc:   "section is member of a group",
										Value: "value & 0x200 != 0",
									},
								},
								{
									Key: "tls",
									Value: InstanceSpec{
										Doc:   "section hold thread-local data",
										Value: "value & 0x400 != 0",
									},
								},
								{
									Key: "ordered",
									Value: InstanceSpec{
										Doc:   "special ordering requirement (Solaris)",
										Value: "value & 0x04000000 != 0",
									},
								},
								{
									Key: "exclude",
									Value: InstanceSpec{
										Doc:   "section is excluded unless referenced or allocated (Solaris)",
										Value: "value & 0x08000000 != 0",
									},
								},
								{
									Key: "mask_os",
									Value: InstanceSpec{
										Doc:   "OS-specific",
										Value: "value & 0x0ff00000 != 0",
									},
								},
								{
									Key: "mask_proc",
									Value: InstanceSpec{
										Doc:   "Processor-specific",
										Value: "value & 0xf0000000 != 0",
									},
								},
							},
						},
					},
					TypeSpec{
						Meta: MetaSpec{
							ID: "dt_flag_1_values",
						},
						Params: ParamsSpec{
							{ID: "value", Type: "u4"},
						},
						Instances: InstancesSpec{
							Instances: []InstanceSpecItem{
								{
									Key: "now",
									Value: InstanceSpec{
										Doc:   "Set RTLD_NOW for this object.",
										Value: "value & 0x00000001 != 0",
									},
								},
								{
									Key: "rtld_global",
									Value: InstanceSpec{
										Doc:   "Set RTLD_GLOBAL for this object.",
										Value: "value & 0x00000002 != 0",
									},
								},
								{
									Key: "group",
									Value: InstanceSpec{
										Doc:   "Set RTLD_GROUP for this object.",
										Value: "value & 0x00000004 != 0",
									},
								},
								{
									Key: "nodelete",
									Value: InstanceSpec{
										Doc:   "Set RTLD_NODELETE for this object.",
										Value: "value & 0x00000008 != 0",
									},
								},
								{
									Key: "loadfltr",
									Value: InstanceSpec{
										Doc:   "Trigger filtee loading at runtime.",
										Value: "value & 0x00000010 != 0",
									},
								},
								{
									Key: "initfirst",
									Value: InstanceSpec{
										Doc:   "Set RTLD_INITFIRST for this object",
										Value: "value & 0x00000020 != 0",
									},
								},
								{
									Key: "noopen",
									Value: InstanceSpec{
										Doc:   "Set RTLD_NOOPEN for this object.",
										Value: "value & 0x00000040 != 0",
									},
								},
								{
									Key: "origin",
									Value: InstanceSpec{
										Doc:   "$ORIGIN must be handled.",
										Value: "value & 0x00000080 != 0",
									},
								},
								{
									Key: "direct",
									Value: InstanceSpec{
										Doc:   "Direct binding enabled.",
										Value: "value & 0x00000100 != 0",
									},
								},
								{
									Key: "trans",
									Value: InstanceSpec{
										Value: "value & 0x00000200 != 0",
									},
								},
								{
									Key: "interpose",
									Value: InstanceSpec{
										Doc:   "Object is used to interpose.",
										Value: "value & 0x00000400 != 0",
									},
								},
								{
									Key: "nodeflib",
									Value: InstanceSpec{
										Doc:   "Ignore default lib search path.",
										Value: "value & 0x00000800 != 0",
									},
								},
								{
									Key: "nodump",
									Value: InstanceSpec{
										Doc:   "Object can't be dldump'ed.",
										Value: "value & 0x00001000 != 0",
									},
								},
								{
									Key: "confalt",
									Value: InstanceSpec{
										Doc:   "Configuration alternative created.",
										Value: "value & 0x00002000 != 0",
									},
								},
								{
									Key: "endfiltee",
									Value: InstanceSpec{
										Doc:   "Filtee terminates filters search.",
										Value: "value & 0x00004000 != 0",
									},
								},
								{
									Key: "dispreldne",
									Value: InstanceSpec{
										Doc:   "Disp reloc applied at build time.",
										Value: "value & 0x00008000 != 0",
									},
								},
								{
									Key: "disprelpnd",
									Value: InstanceSpec{
										Doc:   "Disp reloc applied at run-time.",
										Value: "value & 0x00010000 != 0",
									},
								},
								{
									Key: "nodirect",
									Value: InstanceSpec{
										Doc:   "Object has no-direct binding.",
										Value: "value & 0x00020000 != 0",
									},
								},
								{
									Key: "ignmuldef",
									Value: InstanceSpec{
										Value: "value & 0x00040000 != 0",
									},
								},
								{
									Key: "noksyms",
									Value: InstanceSpec{
										Value: "value & 0x00080000 != 0",
									},
								},
								{
									Key: "nohdr",
									Value: InstanceSpec{
										Value: "value & 0x00100000 != 0",
									},
								},
								{
									Key: "edited",
									Value: InstanceSpec{
										Doc:   "Object is modified after built.",
										Value: "value & 0x00200000 != 0",
									},
								},
								{
									Key: "noreloc",
									Value: InstanceSpec{
										Value: "value & 0x00400000 != 0",
									},
								},
								{
									Key: "symintpose",
									Value: InstanceSpec{
										Doc:   "Object has individual interposers.",
										Value: "value & 0x00800000 != 0",
									},
								},
								{
									Key: "globaudit",
									Value: InstanceSpec{
										Doc:   "Global auditing required.",
										Value: "value & 0x01000000 != 0",
									},
								},
								{
									Key: "singleton",
									Value: InstanceSpec{
										Doc:   "Singleton symbols are used.",
										Value: "value & 0x02000000 != 0",
									},
								},
								{
									Key: "stub",
									Value: InstanceSpec{
										Value: "value & 0x04000000 != 0",
									},
								},
								{
									Key: "pie",
									Value: InstanceSpec{
										Value: "value & 0x08000000 != 0",
									},
								},
							},
						},
					},
					TypeSpec{
						Meta: MetaSpec{
							ID: "endian_elf",
							Endian: EndianSpec{
								SwitchOn: "_root.endian",
								Cases: EndianCaseMapSpec{
									"endian::be": "be",
									"endian::le": "le",
								},
							},
						},
						Seq: AttributesSpec{
							AttributeSpec{
								ID: "e_type",
								Type: AttrTypeSpec{
									Value: "u2",
								},
								Enum: "obj_type",
							},
							AttributeSpec{
								ID: "machine",
								Type: AttrTypeSpec{
									Value: "u2",
								},
								Enum: "machine",
							},
							AttributeSpec{
								ID: "e_version",
								Type: AttrTypeSpec{
									Value: "u4",
								},
							},
							AttributeSpec{
								ID: "entry_point",
								Type: AttrTypeSpec{
									SwitchOn: "_root.bits",
									Cases: TypeCaseMapSpec{
										"bits::b32": "u4",
										"bits::b64": "u8",
									},
								},
							},
							AttributeSpec{
								ID: "program_header_offset",
								Type: AttrTypeSpec{
									SwitchOn: "_root.bits",
									Cases: TypeCaseMapSpec{
										"bits::b32": "u4",
										"bits::b64": "u8",
									},
								},
							},
							AttributeSpec{
								ID: "section_header_offset",
								Type: AttrTypeSpec{
									SwitchOn: "_root.bits",
									Cases: TypeCaseMapSpec{
										"bits::b32": "u4",
										"bits::b64": "u8",
									},
								},
							},
							AttributeSpec{
								ID:   "flags",
								Size: "4",
							},
							AttributeSpec{
								ID: "e_ehsize",
								Type: AttrTypeSpec{
									Value: "u2",
								},
							},
							AttributeSpec{
								ID: "program_header_entry_size",
								Type: AttrTypeSpec{
									Value: "u2",
								},
							},
							AttributeSpec{
								ID: "qty_program_header",
								Type: AttrTypeSpec{
									Value: "u2",
								},
							},
							AttributeSpec{
								ID: "section_header_entry_size",
								Type: AttrTypeSpec{
									Value: "u2",
								},
							},
							AttributeSpec{
								ID: "qty_section_header",
								Type: AttrTypeSpec{
									Value: "u2",
								},
							},
							AttributeSpec{
								ID: "section_names_idx",
								Type: AttrTypeSpec{
									Value: "u2",
								},
							},
						},
						Types: TypesSpec{
							TypeSpec{
								Meta: MetaSpec{
									ID: "program_header",
								},
								Seq: AttributesSpec{
									AttributeSpec{
										ID: "type",
										Type: AttrTypeSpec{
											Value: "u4",
										},
										Enum: "ph_type",
									},
									AttributeSpec{
										ID: "flags64",
										Type: AttrTypeSpec{
											Value: "u4",
										},
										If: "_root.bits == bits::b64",
									},
									AttributeSpec{
										ID: "offset",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "vaddr",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "paddr",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "filesz",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "memsz",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "flags32",
										Type: AttrTypeSpec{
											Value: "u4",
										},
										If: "_root.bits == bits::b32",
									},
									AttributeSpec{
										ID: "align",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
								},

								Instances: InstancesSpec{
									Instances: []InstanceSpecItem{
										{
											Key: "dynamic",
											Value: InstanceSpec{
												Type: AttrTypeSpec{
													Value: "dynamic_section",
												},
												If:   "type == ph_type::dynamic",
												Size: "filesz",
												Pos:  "offset",
												IO:   "_root._io",
											},
										},
										{
											Key: "flags_obj",
											Value: InstanceSpec{
												Type: AttrTypeSpec{
													Value: "phdr_type_flags(flags64|flags32)",
												},
											},
										},
									},
								},
							},
							TypeSpec{
								Meta: MetaSpec{
									ID: "section_header",
								},
								Seq: AttributesSpec{
									AttributeSpec{
										ID: "ofs_name",
										Type: AttrTypeSpec{
											Value: "u4",
										},
									},
									AttributeSpec{
										ID: "type",
										Type: AttrTypeSpec{
											Value: "u4",
										},
										Enum: "sh_type",
									},
									AttributeSpec{
										ID: "flags",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "addr",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "ofs_body",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "len_body",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "linked_section_idx",
										Type: AttrTypeSpec{
											Value: "u4",
										},
									},
									AttributeSpec{
										ID:   "info",
										Size: "4",
									},
									AttributeSpec{
										ID: "align",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "entry_size",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
								},

								Instances: InstancesSpec{
									Instances: []InstanceSpecItem{
										{
											Key: "body",
											Value: InstanceSpec{
												Type: AttrTypeSpec{
													SwitchOn: "type",
													Cases: TypeCaseMapSpec{
														"sh_type::dynamic": "dynamic_section",
														"sh_type::dynstr":  "strings_struct",
														"sh_type::dynsym":  "dynsym_section",
														"sh_type::strtab":  "strings_struct",
													},
												},
												Size: "len_body",
												Pos:  "ofs_body",
												IO:   "_root._io",
											},
										},
										{
											Key: "name",
											Value: InstanceSpec{
												Type: AttrTypeSpec{
													Value: "strz",
												},
												Encoding: "ASCII",
												Pos:      "ofs_name",
												IO:       "_root.header.strings._io",
											},
										},
										{
											Key: "flags_obj",
											Value: InstanceSpec{
												Type: AttrTypeSpec{
													Value: "section_header_flags(flags)",
												},
											},
										},
									},
								},
							},
							TypeSpec{
								Meta: MetaSpec{
									ID: "strings_struct",
								},
								Seq: AttributesSpec{
									AttributeSpec{
										ID: "entries",
										Type: AttrTypeSpec{
											Value: "strz",
										},
										Repeat:   "eos",
										Encoding: "ASCII",
									},
								},
							},
							TypeSpec{
								Meta: MetaSpec{
									ID: "dynamic_section",
								},
								Seq: AttributesSpec{
									AttributeSpec{
										ID: "entries",
										Type: AttrTypeSpec{
											Value: "dynamic_section_entry",
										},
										Repeat: "eos",
									},
								},
							},
							TypeSpec{
								Meta: MetaSpec{
									ID: "dynamic_section_entry",
								},
								Seq: AttributesSpec{
									AttributeSpec{
										ID: "tag",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
									AttributeSpec{
										ID: "value_or_ptr",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "u4",
												"bits::b64": "u8",
											},
										},
									},
								},

								Instances: InstancesSpec{
									Instances: []InstanceSpecItem{
										{
											Key: "tag_enum",
											Value: InstanceSpec{
												Enum:  "dynamic_array_tags",
												Value: "tag",
											},
										},
										{
											Key: "flag_1_values",
											Value: InstanceSpec{
												Type: AttrTypeSpec{
													Value: "dt_flag_1_values(value_or_ptr)",
												},
												If: "tag_enum == dynamic_array_tags::flags_1",
											},
										},
									},
								},
							},
							TypeSpec{
								Meta: MetaSpec{
									ID: "dynsym_section",
								},
								Seq: AttributesSpec{
									AttributeSpec{
										ID: "entries",
										Type: AttrTypeSpec{
											SwitchOn: "_root.bits",
											Cases: TypeCaseMapSpec{
												"bits::b32": "dynsym_section_entry32",
												"bits::b64": "dynsym_section_entry64",
											},
										},
										Repeat: "eos",
									},
								},
							},
							TypeSpec{
								Meta: MetaSpec{
									ID: "dynsym_section_entry32",
								},
								Seq: AttributesSpec{
									AttributeSpec{
										ID: "name_offset",
										Type: AttrTypeSpec{
											Value: "u4",
										},
									},
									AttributeSpec{
										ID: "value",
										Type: AttrTypeSpec{
											Value: "u4",
										},
									},
									AttributeSpec{
										ID: "size",
										Type: AttrTypeSpec{
											Value: "u4",
										},
									},
									AttributeSpec{
										ID: "info",
										Type: AttrTypeSpec{
											Value: "u1",
										},
									},
									AttributeSpec{
										ID: "other",
										Type: AttrTypeSpec{
											Value: "u1",
										},
									},
									AttributeSpec{
										ID: "shndx",
										Type: AttrTypeSpec{
											Value: "u2",
										},
									},
								},
							},
							TypeSpec{
								Meta: MetaSpec{
									ID: "dynsym_section_entry64",
								},
								Seq: AttributesSpec{
									AttributeSpec{
										ID: "name_offset",
										Type: AttrTypeSpec{
											Value: "u4",
										},
									},
									AttributeSpec{
										ID: "info",
										Type: AttrTypeSpec{
											Value: "u1",
										},
									},
									AttributeSpec{
										ID: "other",
										Type: AttrTypeSpec{
											Value: "u1",
										},
									},
									AttributeSpec{
										ID: "shndx",
										Type: AttrTypeSpec{
											Value: "u2",
										},
									},
									AttributeSpec{
										ID: "value",
										Type: AttrTypeSpec{
											Value: "u8",
										},
									},
									AttributeSpec{
										ID: "size",
										Type: AttrTypeSpec{
											Value: "u8",
										},
									},
								},
							},
						},
						Instances: InstancesSpec{
							Instances: []InstanceSpecItem{
								{
									Key: "program_headers",
									Value: InstanceSpec{
										Type: AttrTypeSpec{
											Value: "program_header",
										},
										Repeat:     "expr",
										RepeatExpr: "qty_program_header",
										Size:       "program_header_entry_size",
										Pos:        "program_header_offset",
									},
								},
								{
									Key: "section_headers",
									Value: InstanceSpec{
										Type: AttrTypeSpec{
											Value: "section_header",
										},
										Repeat:     "expr",
										RepeatExpr: "qty_section_header",
										Size:       "section_header_entry_size",
										Pos:        "section_header_offset",
									},
								},
								{
									Key: "strings",
									Value: InstanceSpec{
										Type: AttrTypeSpec{
											Value: "strings_struct",
										},
										Size: "section_headers[section_names_idx].len_body",
										Pos:  "section_headers[section_names_idx].ofs_body",
									},
								},
							},
						},
					},
				},
				Enums: EnumsSpec{
					EnumSpec{
						ID: "bits",
						Values: EnumValuePairsSpec{
							{"1", EnumValueSpec{ID: "b32"}},
							{"2", EnumValueSpec{ID: "b64"}},
						},
					},
					EnumSpec{
						ID: "endian",
						Values: EnumValuePairsSpec{
							{"1", EnumValueSpec{ID: "le"}},
							{"2", EnumValueSpec{ID: "be"}},
						},
					},
					EnumSpec{
						ID: "os_abi",
						Values: EnumValuePairsSpec{
							{"0", EnumValueSpec{ID: "system_v"}},
							{"1", EnumValueSpec{ID: "hp_ux"}},
							{"2", EnumValueSpec{ID: "netbsd"}},
							{"3", EnumValueSpec{ID: "gnu"}},
							{"6", EnumValueSpec{ID: "solaris"}},
							{"7", EnumValueSpec{ID: "aix"}},
							{"8", EnumValueSpec{ID: "irix"}},
							{"9", EnumValueSpec{ID: "freebsd"}},
							{"0xa", EnumValueSpec{ID: "tru64"}},
							{"0xb", EnumValueSpec{ID: "modesto"}},
							{"0xc", EnumValueSpec{ID: "openbsd"}},
							{"0xd", EnumValueSpec{ID: "openvms"}},
							{"0xe", EnumValueSpec{ID: "nsk"}},
							{"0xf", EnumValueSpec{ID: "aros"}},
							{"0x10", EnumValueSpec{ID: "fenixos"}},
							{"0x11", EnumValueSpec{ID: "cloudabi"}},
							{"0x12", EnumValueSpec{ID: "openvos"}},
						},
					},
					EnumSpec{
						ID: "obj_type",
						Values: EnumValuePairsSpec{
							{"1", EnumValueSpec{ID: "relocatable"}},
							{"2", EnumValueSpec{ID: "executable"}},
							{"3", EnumValueSpec{ID: "shared"}},
							{"4", EnumValueSpec{ID: "core"}},
						},
					},
					EnumSpec{
						ID: "machine",
						Values: EnumValuePairsSpec{
							{"0x00", EnumValueSpec{ID: "not_set"}},
							{"0x02", EnumValueSpec{ID: "sparc"}},
							{"0x03", EnumValueSpec{ID: "x86"}},
							{"0x08", EnumValueSpec{ID: "mips"}},
							{"0x14", EnumValueSpec{ID: "powerpc"}},
							{"0x28", EnumValueSpec{ID: "arm"}},
							{"0x2A", EnumValueSpec{ID: "superh"}},
							{"0x32", EnumValueSpec{ID: "ia_64"}},
							{"0x3E", EnumValueSpec{ID: "x86_64"}},
							{"0xB7", EnumValueSpec{ID: "aarch64"}},
							{"0xF3", EnumValueSpec{ID: "riscv"}},
							{"0xF7", EnumValueSpec{ID: "bpf"}},
						},
					},
					EnumSpec{
						ID: "ph_type",
						Values: EnumValuePairsSpec{
							{"0", EnumValueSpec{ID: "null_type"}},
							{"1", EnumValueSpec{ID: "load"}},
							{"2", EnumValueSpec{ID: "dynamic"}},
							{"3", EnumValueSpec{ID: "interp"}},
							{"4", EnumValueSpec{ID: "note"}},
							{"5", EnumValueSpec{ID: "shlib"}},
							{"6", EnumValueSpec{ID: "phdr"}},
							{"7", EnumValueSpec{ID: "tls"}},
							{"0x65041580", EnumValueSpec{ID: "pax_flags"}},
							{"0x6fffffff", EnumValueSpec{ID: "hios"}},
							{"0x70000001", EnumValueSpec{ID: "arm_exidx"}},
							{"0x6474e550", EnumValueSpec{ID: "gnu_eh_frame"}},
							{"0x6474e551", EnumValueSpec{ID: "gnu_stack"}},
							{"0x6474e552", EnumValueSpec{ID: "gnu_relro"}},
						},
					},
					EnumSpec{
						ID: "sh_type",
						Values: EnumValuePairsSpec{
							{"0", EnumValueSpec{ID: "null_type"}},
							{"1", EnumValueSpec{ID: "progbits"}},
							{"2", EnumValueSpec{ID: "symtab"}},
							{"3", EnumValueSpec{ID: "strtab"}},
							{"4", EnumValueSpec{ID: "rela"}},
							{"5", EnumValueSpec{ID: "hash"}},
							{"6", EnumValueSpec{ID: "dynamic"}},
							{"7", EnumValueSpec{ID: "note"}},
							{"8", EnumValueSpec{ID: "nobits"}},
							{"9", EnumValueSpec{ID: "rel"}},
							{"10", EnumValueSpec{ID: "shlib"}},
							{"11", EnumValueSpec{ID: "dynsym"}},
							{"14", EnumValueSpec{ID: "init_array"}},
							{"15", EnumValueSpec{ID: "fini_array"}},
							{"16", EnumValueSpec{ID: "preinit_array"}},
							{"17", EnumValueSpec{ID: "group"}},
							{"18", EnumValueSpec{ID: "symtab_shndx"}},
							{"0x6fffffef", EnumValueSpec{ID: "sunw_capchain"}},
							{"0x6ffffff0", EnumValueSpec{ID: "sunw_capinfo"}},
							{"0x6ffffff1", EnumValueSpec{ID: "sunw_symsort"}},
							{"0x6ffffff2", EnumValueSpec{ID: "sunw_tlssort"}},
							{"0x6ffffff3", EnumValueSpec{ID: "sunw_ldynsym"}},
							{"0x6ffffff4", EnumValueSpec{ID: "sunw_dof"}},
							{"0x6ffffff5", EnumValueSpec{ID: "sunw_cap"}},
							{"0x6ffffff6", EnumValueSpec{ID: "sunw_signature"}},
							{"0x6ffffff7", EnumValueSpec{ID: "sunw_annotate"}},
							{"0x6ffffff8", EnumValueSpec{ID: "sunw_debugstr"}},
							{"0x6ffffff9", EnumValueSpec{ID: "sunw_debug"}},
							{"0x6ffffffa", EnumValueSpec{ID: "sunw_move"}},
							{"0x6ffffffb", EnumValueSpec{ID: "sunw_comdat"}},
							{"0x6ffffffc", EnumValueSpec{ID: "sunw_syminfo"}},
							{"0x6ffffffd", EnumValueSpec{ID: "sunw_verdef"}},
							{"0x6ffffffe", EnumValueSpec{ID: "sunw_verneed"}},
							{"0x6fffffff", EnumValueSpec{ID: "sunw_versym"}},
							{"0x70000000", EnumValueSpec{ID: "sparc_gotdata"}},
							{"0x70000001", EnumValueSpec{ID: "amd64_unwind"}},
							{"0x70000001", EnumValueSpec{ID: "arm_exidx"}},
							{"0x70000002", EnumValueSpec{ID: "arm_preemptmap"}},
							{"0x70000003", EnumValueSpec{ID: "arm_attributes"}},
						},
					},
					EnumSpec{
						ID: "dynamic_array_tags",
						Values: EnumValuePairsSpec{
							{"0", EnumValueSpec{ID: "null"}},
							{"1", EnumValueSpec{ID: "needed"}},
							{"2", EnumValueSpec{ID: "pltrelsz"}},
							{"3", EnumValueSpec{ID: "pltgot"}},
							{"4", EnumValueSpec{ID: "hash"}},
							{"5", EnumValueSpec{ID: "strtab"}},
							{"6", EnumValueSpec{ID: "symtab"}},
							{"7", EnumValueSpec{ID: "rela"}},
							{"8", EnumValueSpec{ID: "relasz"}},
							{"9", EnumValueSpec{ID: "relaent"}},
							{"10", EnumValueSpec{ID: "strsz"}},
							{"11", EnumValueSpec{ID: "syment"}},
							{"12", EnumValueSpec{ID: "init"}},
							{"13", EnumValueSpec{ID: "fini"}},
							{"14", EnumValueSpec{ID: "soname"}},
							{"15", EnumValueSpec{ID: "rpath"}},
							{"16", EnumValueSpec{ID: "symbolic"}},
							{"17", EnumValueSpec{ID: "rel"}},
							{"18", EnumValueSpec{ID: "relsz"}},
							{"19", EnumValueSpec{ID: "relent"}},
							{"20", EnumValueSpec{ID: "pltrel"}},
							{"21", EnumValueSpec{ID: "debug"}},
							{"22", EnumValueSpec{ID: "textrel"}},
							{"23", EnumValueSpec{ID: "jmprel"}},
							{"24", EnumValueSpec{ID: "bind_now"}},
							{"25", EnumValueSpec{ID: "init_array"}},
							{"26", EnumValueSpec{ID: "fini_array"}},
							{"27", EnumValueSpec{ID: "init_arraysz"}},
							{"28", EnumValueSpec{ID: "fini_arraysz"}},
							{"29", EnumValueSpec{ID: "runpath"}},
							{"30", EnumValueSpec{ID: "flags"}},
							{"32", EnumValueSpec{ID: "encoding"}},
							{"32", EnumValueSpec{ID: "preinit_array"}},
							{"33", EnumValueSpec{ID: "preinit_arraysz"}},
							{"34", EnumValueSpec{ID: "maxpostags"}},
							{"0x6000000d", EnumValueSpec{ID: "loos"}},
							{"0x6000000d", EnumValueSpec{ID: "sunw_auxiliary"}},
							{"0x6000000e", EnumValueSpec{ID: "sunw_rtldinf"}},
							{"0x6000000e", EnumValueSpec{ID: "sunw_filter"}},
							{"0x60000010", EnumValueSpec{ID: "sunw_cap"}},
							{"0x60000011", EnumValueSpec{ID: "sunw_symtab"}},
							{"0x60000012", EnumValueSpec{ID: "sunw_symsz"}},
							{"0x60000013", EnumValueSpec{ID: "sunw_encoding"}},
							{"0x60000013", EnumValueSpec{ID: "sunw_sortent"}},
							{"0x60000014", EnumValueSpec{ID: "sunw_symsort"}},
							{"0x60000015", EnumValueSpec{ID: "sunw_symsortsz"}},
							{"0x60000016", EnumValueSpec{ID: "sunw_tlssort"}},
							{"0x60000017", EnumValueSpec{ID: "sunw_tlssortsz"}},
							{"0x60000018", EnumValueSpec{ID: "sunw_capinfo"}},
							{"0x60000019", EnumValueSpec{ID: "sunw_strpad"}},
							{"0x6000001a", EnumValueSpec{ID: "sunw_capchain"}},
							{"0x6000001b", EnumValueSpec{ID: "sunw_ldmach"}},
							{"0x6000001d", EnumValueSpec{ID: "sunw_capchainent"}},
							{"0x6000001f", EnumValueSpec{ID: "sunw_capchainsz"}},
							{"0x6ffff000", EnumValueSpec{ID: "hios"}},
							{"0x6ffffd00", EnumValueSpec{ID: "valrnglo"}},
							{"0x6ffffdf5", EnumValueSpec{ID: "gnu_prelinked"}},
							{"0x6ffffdf6", EnumValueSpec{ID: "gnu_conflictsz"}},
							{"0x6ffffdf7", EnumValueSpec{ID: "gnu_liblistsz"}},
							{"0x6ffffdf8", EnumValueSpec{ID: "checksum"}},
							{"0x6ffffdf9", EnumValueSpec{ID: "pltpadsz"}},
							{"0x6ffffdfa", EnumValueSpec{ID: "moveent"}},
							{"0x6ffffdfb", EnumValueSpec{ID: "movesz"}},
							{"0x6ffffdfc", EnumValueSpec{ID: "feature_1"}},
							{"0x6ffffdfd", EnumValueSpec{ID: "posflag_1"}},
							{"0x6ffffdfe", EnumValueSpec{ID: "syminsz"}},
							{"0x6ffffdff", EnumValueSpec{ID: "syminent"}},
							{"0x6ffffdff", EnumValueSpec{ID: "valrnghi"}},
							{"0x6ffffe00", EnumValueSpec{ID: "addrrnglo"}},
							{"0x6ffffef5", EnumValueSpec{ID: "gnu_hash"}},
							{"0x6ffffef6", EnumValueSpec{ID: "tlsdesc_plt"}},
							{"0x6ffffef7", EnumValueSpec{ID: "tlsdesc_got"}},
							{"0x6ffffef8", EnumValueSpec{ID: "gnu_conflict"}},
							{"0x6ffffef9", EnumValueSpec{ID: "gnu_liblist"}},
							{"0x6ffffefa", EnumValueSpec{ID: "config"}},
							{"0x6ffffefb", EnumValueSpec{ID: "depaudit"}},
							{"0x6ffffefc", EnumValueSpec{ID: "audit"}},
							{"0x6ffffefd", EnumValueSpec{ID: "pltpad"}},
							{"0x6ffffefe", EnumValueSpec{ID: "movetab"}},
							{"0x6ffffeff", EnumValueSpec{ID: "syminfo"}},
							{"0x6ffffeff", EnumValueSpec{ID: "addrrnghi"}},
							{"0x6ffffff0", EnumValueSpec{ID: "versym"}},
							{"0x6ffffff9", EnumValueSpec{ID: "relacount"}},
							{"0x6ffffffa", EnumValueSpec{ID: "relcount"}},
							{"0x6ffffffb", EnumValueSpec{ID: "flags_1"}},
							{"0x6ffffffc", EnumValueSpec{ID: "verdef"}},
							{"0x6ffffffd", EnumValueSpec{ID: "verdefnum"}},
							{"0x6ffffffe", EnumValueSpec{ID: "verneed"}},
							{"0x6fffffff", EnumValueSpec{ID: "verneednum"}},
							{"0x70000000", EnumValueSpec{ID: "loproc"}},
							{"0x70000001", EnumValueSpec{ID: "sparc_register"}},
							{"0x7ffffffd", EnumValueSpec{ID: "auxiliary"}},
							{"0x7ffffffe", EnumValueSpec{ID: "used"}},
							{"0x7fffffff", EnumValueSpec{ID: "filter"}},
							{"0x7fffffff", EnumValueSpec{ID: "hiproc"}},
						},
					},
				},
				DocRef: DocRefSpec{"https://sourceware.org/git/?p=glibc.git;a=blob;f=elf/elf.h;hb=HEAD"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.filename,
			func(t *testing.T) {
				contents, err := os.ReadFile("testdata/" + test.filename)
				require.NoError(t, err)

				actual := TypeSpec{}
				require.NoError(t, yaml.Unmarshal(contents, &actual))
				assert.Equal(t, test.expected, actual)

				roundtrip := TypeSpec{}
				marshaled, err := yaml.Marshal(actual)
				require.NoError(t, err)
				require.NoError(t, yaml.Unmarshal(marshaled, &roundtrip))
				assert.Equal(t, test.expected, roundtrip)
			})
	}
}

func BenchmarkFullSchemaUnmarshal(b *testing.B) {
	b.StopTimer()
	contents, err := os.ReadFile("testdata/elf.ksy")
	require.Nil(b, err)
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		actual := TypeSpec{}
		_ = yaml.Unmarshal(contents, &actual)
	}
}

func BenchmarkFullSchemaMarshal(b *testing.B) {
	b.StopTimer()
	contents, err := os.ReadFile("testdata/elf.ksy")
	require.Nil(b, err)
	actual := TypeSpec{}
	require.Nil(b, yaml.Unmarshal(contents, &actual))
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_, _ = yaml.Marshal(actual)
	}
}
