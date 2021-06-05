package ksy

// XrefSpec represents the type of meta/xref.
type XrefSpec map[string]interface{}

// MetaSpec represents a meta block.
// #/definitions/MetaSpec
type MetaSpec struct {
	ID            Identifier  `yaml:"id,omitempty"`
	Title         string      `yaml:"title,omitempty"`
	Application   MultiString `yaml:"application,omitempty"`
	FileExtension MultiString `yaml:"file-extension,omitempty"`
	License       string      `yaml:"license,omitempty"`
	KSVersion     string      `yaml:"ks-version,omitempty"`
	KSDebug       bool        `yaml:"ks-debug,omitempty"`
	KSOpaqueTypes bool        `yaml:"ks-opaque-types,omitempty"`
	Imports       []string    `yaml:"imports,omitempty"`
	Encoding      string      `yaml:"encoding,omitempty"`
	Endian        EndianSpec  `yaml:"endian,omitempty"`
	Xref          XrefSpec    `yaml:"xref,omitempty"`
}
