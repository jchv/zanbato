package resolve

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/jchv/zanbato/kaitai"
)

type Resolver interface {
	Resolve(from, to string) (string, *kaitai.Struct, error)
}

type fileResolver struct {
	cache       map[string]*kaitai.Struct
	open        func(name string) (io.ReadCloser, error)
	importPaths []string // directories to search for absolute imports (starting with /)
}

func NewOSResolver() Resolver {
	return &fileResolver{
		cache: make(map[string]*kaitai.Struct),
		open: func(name string) (io.ReadCloser, error) {
			return os.Open(name)
		},
	}
}

// NewOSResolverWithPaths creates a resolver that also searches the given
// directories for absolute imports (import paths starting with /).
func NewOSResolverWithPaths(importPaths []string) Resolver {
	return &fileResolver{
		cache:       make(map[string]*kaitai.Struct),
		importPaths: importPaths,
		open: func(name string) (io.ReadCloser, error) {
			return os.Open(name)
		},
	}
}

func NewFSResolver(fs fs.FS) Resolver {
	return &fileResolver{
		cache:       make(map[string]*kaitai.Struct),
		importPaths: []string{"."},
		open: func(name string) (io.ReadCloser, error) {
			return fs.Open(name)
		},
	}
}

func (resolver *fileResolver) Resolve(from, to string) (string, *kaitai.Struct, error) {
	basename := to
	isAbsolute := len(to) > 0 && to[0] == '/'
	if isAbsolute {
		// Absolute imports: search import paths
		basename = to[1:] // strip leading /
	} else if from != "" {
		basename = path.Join(path.Dir(from), to)
	}
	if cachedStruct, ok := resolver.cache[basename]; ok {
		return basename, cachedStruct, nil
	}

	var candidates []string
	if isAbsolute {
		// For absolute imports, search each import path
		for _, dir := range resolver.importPaths {
			full := path.Join(dir, basename)
			candidates = append(candidates, full+".ksy", full)
		}
	} else {
		candidates = []string{basename + ".ksy", basename}
	}

	for _, name := range candidates {
		file, err := resolver.open(name)
		if err != nil {
			continue
		}
		defer func() { _ = file.Close() }()
		struc, err := kaitai.ParseStruct(file)
		if err != nil {
			return "", nil, fmt.Errorf("error loading %q: %w", name, err)
		}
		// Use the full path as basename so relative imports from this file
		// resolve correctly relative to its actual filesystem location.
		resolvedBasename := name
		if path.Ext(resolvedBasename) == ".ksy" {
			resolvedBasename = resolvedBasename[:len(resolvedBasename)-4]
		}
		resolver.cache[resolvedBasename] = struc
		return resolvedBasename, struc, nil
	}
	return "", nil, fmt.Errorf("failed to load struct %s from %s (checked %v)", to, from, candidates)
}
