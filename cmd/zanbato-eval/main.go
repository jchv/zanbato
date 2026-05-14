package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/jchv/zanbato/kaitai/eval"
	"github.com/jchv/zanbato/kaitai/resolve"
)

// treeJSON is a JSON-serializable representation of a Node tree.
type treeJSON struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Kind     string      `json:"kind"`
	Value    any         `json:"value,omitempty"`
	Range    *eval.Range `json:"range,omitempty"`
	Error    string      `json:"error,omitempty"`
	Children []*treeJSON `json:"children,omitempty"`
}

func nodeToJSON(n *eval.Node) *treeJSON {
	j := &treeJSON{
		Name: n.Name(),
		Path: n.Path().String(),
	}

	if err := n.Resolve(); err != nil {
		j.Error = err.Error()
		return j
	}

	v, _ := n.Value()
	j.Kind = v.Kind.String()

	switch v.Kind {
	case eval.KindInt:
		j.Value = v.Int
	case eval.KindUint:
		j.Value = v.Uint
	case eval.KindFloat:
		j.Value = v.Float
	case eval.KindBool:
		j.Value = v.Bool
	case eval.KindBytes:
		j.Value = v.Bytes
	case eval.KindStr:
		j.Value = v.Str
	case eval.KindEnum:
		j.Value = map[string]any{"int": v.Int, "enum": v.EnumName, "label": v.EnumLabel}
	}

	r, _ := n.ByteRange()
	if r.StartIndex != r.EndIndex {
		j.Range = &r
	}

	// Expand children for structs
	if v.Kind == eval.KindStruct {
		for _, child := range n.Fields() {
			j.Children = append(j.Children, nodeToJSON(child))
		}
	}

	// Expand items for arrays
	if v.Kind == eval.KindArray {
		items, _ := n.Items()
		for _, item := range items {
			j.Children = append(j.Children, nodeToJSON(item))
		}
	}

	return j
}

func main() {
	flag.Parse()
	if flag.NArg() != 2 {
		log.Fatalln("Wrong number of arguments; pass your root .ksy path and a binary file to read.")
	}
	rootname := flag.Arg(0)
	filename := flag.Arg(1)
	resolver := resolve.NewOSResolver()
	basename, struc, err := resolver.Resolve("", rootname)
	if err != nil {
		log.Fatalf("error resolving root struct: %v", err)
	}
	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("error opening file %q: %v", filename, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("warning: error closing file %q: %v", filename, err)
		}
	}()
	stream := eval.NewStream(f)
	tree, err := eval.NewTree(resolver, basename, struc, stream)
	if err != nil {
		log.Fatalf("error creating tree: %v", err)
	}

	result := nodeToJSON(tree.Root())
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "\t")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		log.Fatalf("error encoding json: %v", err)
	}
}
