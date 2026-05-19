//go:build js && wasm

// zanbato-wasm is the WebAssembly entry point for the zanbato runtime
// evaluator. It exposes a tiny global API (`globalThis.zanbato`) that a Web
// Worker can call to load KSY definitions and parse binary buffers against
// them.
//
// API:
//
//	zanbato.loadKsys(files: Array<{name: string, source: string}>)
//	    -> {ok: true} | {ok: false, error: string}
//	zanbato.parse(rootName: string, data: Uint8Array)
//	    -> {ok: true, tree: string (JSON)} | {ok: false, error: string}
//
// loadKsys atomically replaces the in-memory VFS with the supplied set of
// files; each entry's `name` becomes a VFS path with `.ksy` appended.
// Callers are expected to pass the full transitive import graph.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"syscall/js"
	"testing/fstest"

	"github.com/jchv/zanbato/kaitai/eval"
	"github.com/jchv/zanbato/kaitai/resolve"
)

// vfs holds the in-memory KSY files that the worker has loaded. Keys are
// VFS paths like "main.ksy" or "subdir/helpers.ksy".
var vfs = fstest.MapFS{}

func main() {
	zanbato := js.Global().Get("Object").New()
	zanbato.Set("loadKsys", js.FuncOf(loadKsys))
	zanbato.Set("parse", js.FuncOf(parse))
	js.Global().Set("zanbato", zanbato)

	// Keep the runtime alive so the registered functions remain callable.
	select {}
}

func loadKsys(_ js.Value, args []js.Value) (ret any) {
	defer func() {
		if r := recover(); r != nil {
			ret = errResult(fmt.Sprintf("panic: %v", r))
		}
	}()
	if len(args) != 1 {
		return errResult("loadKsys: expected (files)")
	}
	files := args[0]
	if files.Type() != js.TypeObject {
		return errResult("loadKsys: files must be an array")
	}
	n := files.Get("length").Int()
	newVfs := fstest.MapFS{}
	for i := range n {
		entry := files.Index(i)
		name := entry.Get("name").String()
		source := entry.Get("source").String()
		newVfs[name+".ksy"] = &fstest.MapFile{Data: []byte(source)}
	}
	vfs = newVfs
	return okResult(nil)
}

func parse(_ js.Value, args []js.Value) (ret any) {
	defer func() {
		if r := recover(); r != nil {
			ret = errResult(fmt.Sprintf("panic: %v", r))
		}
	}()
	if len(args) != 2 {
		return errResult("parse: expected (rootName, data)")
	}
	rootName := args[0].String()
	dataJS := args[1]

	n := dataJS.Get("length").Int()
	data := make([]byte, n)
	js.CopyBytesToGo(data, dataJS)

	resolver := resolve.NewFSResolver(vfs)
	basename, struc, err := resolver.Resolve("", rootName)
	if err != nil {
		return errResult(err.Error())
	}

	stream := eval.NewStream(bytes.NewReader(data))
	tree, err := eval.NewTree(resolver, basename, struc, stream)
	if err != nil {
		return errResult(err.Error())
	}

	root := nodeToJSON(tree.Root())
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return errResult(err.Error())
	}

	out := js.Global().Get("Object").New()
	out.Set("ok", true)
	out.Set("tree", buf.String())
	return out
}

type treeJSON struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Kind     string      `json:"kind,omitempty"`
	TypeName string      `json:"typeName,omitempty"`
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
	// Surface the schema type name for struct-typed nodes.
	if v.Kind == eval.KindStruct {
		if s := n.StructSchema(); s != nil {
			j.TypeName = string(s.ID)
		}
	}
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
		// Go's encoding/json serializes []byte as base64, which is awkward
		// to display on the frontend. []int is a bit more convenient.
		out := make([]int, len(v.Bytes))
		for i, b := range v.Bytes {
			out[i] = int(b)
		}
		j.Value = out
	case eval.KindStr:
		j.Value = v.Str
	case eval.KindEnum:
		j.Value = map[string]any{
			"int":   v.Int,
			"enum":  v.EnumName,
			"label": v.EnumLabel,
		}
	}
	r, _ := n.ByteRange()
	if r.StartIndex != r.EndIndex {
		j.Range = &r
	}
	if v.Kind == eval.KindStruct {
		for _, child := range n.Fields() {
			j.Children = append(j.Children, nodeToJSON(child))
		}
	}
	if v.Kind == eval.KindArray {
		items, _ := n.Items()
		for _, item := range items {
			j.Children = append(j.Children, nodeToJSON(item))
		}
	}
	return j
}

func okResult(value any) js.Value {
	o := js.Global().Get("Object").New()
	o.Set("ok", true)
	if value != nil {
		o.Set("value", js.ValueOf(value))
	}
	return o
}

func errResult(msg string) js.Value {
	o := js.Global().Get("Object").New()
	o.Set("ok", false)
	o.Set("error", msg)
	return o
}
