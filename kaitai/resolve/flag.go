package resolve

import (
	"flag"
	"strings"
)

// ImportPathsFlag is a flag.Value that accumulates repeated -I path values
// into the slice of additional import-search paths used by the resolver.
type ImportPathsFlag []string

// String implements flag.Value.
func (f *ImportPathsFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, ", ")
}

// Set implements flag.Value. Each invocation appends one path.
func (f *ImportPathsFlag) Set(s string) error {
	*f = append(*f, s)
	return nil
}

// RegisterImportPathsFlag binds a repeatable -I flag on fs that gathers
// additional import-search paths. Pass flag.CommandLine when using the
// default flag set. The returned pointer is populated by flag.Parse and
// can be passed directly to NewOSResolverWithPaths.
func RegisterImportPathsFlag(fs *flag.FlagSet) *ImportPathsFlag {
	paths := &ImportPathsFlag{}
	fs.Var(paths, "I", "Additional import search path (repeatable)")
	return paths
}
