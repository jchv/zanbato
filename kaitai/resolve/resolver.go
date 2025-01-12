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
	cache map[string]*kaitai.Struct
	open  func(name string) (io.ReadCloser, error)
}

func NewOSResolver() Resolver {
	return &fileResolver{
		cache: make(map[string]*kaitai.Struct),
		open: func(name string) (io.ReadCloser, error) {
			return os.Open(name)
		},
	}
}

func NewFSResolver(fs fs.FS) Resolver {
	return &fileResolver{
		cache: make(map[string]*kaitai.Struct),
		open: func(name string) (io.ReadCloser, error) {
			return fs.Open(name)
		},
	}
}

func (resolver *fileResolver) Resolve(from, to string) (string, *kaitai.Struct, error) {
	basename := to
	if from != "" {
		basename = path.Join(path.Dir(from), to)
	}
	if cachedStruct, ok := resolver.cache[basename]; ok {
		return basename, cachedStruct, nil
	}
	candidates := []string{basename + ".ksy", basename}
	for _, name := range candidates {
		file, err := resolver.open(name)
		if err != nil {
			continue
		}
		defer file.Close()
		struc, err := kaitai.ParseStruct(file)
		if err != nil {
			return "", nil, fmt.Errorf("error loading %q: %w", name, err)
		}
		resolver.cache[basename] = struc
		return basename, struc, nil
	}
	return "", nil, fmt.Errorf("failed to load struct %s from %s (checked %v)", to, from, candidates)
}
