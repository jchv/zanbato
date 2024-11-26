package eval

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/jchv/zanbato/kaitai/types"
)

type Evaluator struct {
	resolver    resolve.Resolver
	stream      *Stream
	context     *engine.EvalContext
	annotations []Annotation
	endian      types.EndianKind
	path        []PathItem
}

// NewEvaluator constructs a new evaluator.
func NewEvaluator(resolver resolve.Resolver, stream *Stream) *Evaluator {
	return &Evaluator{
		resolver: resolver,
		stream:   stream,
		context:  engine.NewEvalContext(engine.NewContext()),
	}
}

// Evaluate emits Go code for the given kaitai struct.
func (e *Evaluator) Evaluate(inputname string, s *kaitai.Struct) []Annotation {
	e.resolveImports(inputname, s)
	e.root(s)
	annotations := e.annotations
	e.annotations = nil
	return annotations
}

func (e *Evaluator) resolveImports(inputname string, s *kaitai.Struct) {
	// Pivot stack to new root
	root := engine.NewStructValueSymbol(engine.NewStructTypeSymbol(s, nil), nil)
	e.context.AddGlobalType(string(s.ID), root.Type)
	e.context.AddModuleType(string(s.ID), root.Type)
	oldContext := e.context.Context
	e.context.SetContext(e.context.WithModuleRoot(root).WithLocalRoot(root))

	// Recursively import types into the current root
	for _, importname := range s.Meta.Imports {
		inputname, s, err := e.resolver.Resolve(inputname, importname)
		if err != nil {
			panic(err)
		}
		e.resolveImports(inputname, s)
	}

	// Pivot back to old root
	e.context.SetContext(oldContext)
}

func (e *Evaluator) root(s *kaitai.Struct) {
	// Pivot stack to new root
	root := engine.NewStructValueSymbol(engine.NewStructTypeSymbol(s, nil), nil)
	e.context.AddGlobalType(string(s.ID), root.Type)
	e.context.AddModuleType(string(s.ID), root.Type)
	oldContext := e.context.Context
	e.context.SetContext(e.context.WithModuleRoot(root).WithLocalRoot(root).WithStream(engine.NewRuntimeStreamValue(e.stream)))

	e.struc(root)

	// Pivot back to old root
	e.context.SetContext(oldContext)
}

func (e *Evaluator) setEndian(endian types.Endian) {
	switch endian.Kind {
	case types.BigEndian, types.LittleEndian:
		e.endian = endian.Kind
	case types.SwitchEndian:
		switchVal, err := engine.Evaluate(e.context, endian.SwitchOn)
		if err != nil {
			panic(err)
		}
		for caseEx, endian := range endian.Cases {
			caseVal, err := engine.Evaluate(e.context, expr.MustParseExpr(caseEx))
			cmp, err := engine.Compare(switchVal, caseVal, engine.CompareEqual)
			if err != nil {
				panic(err)
			}
			if cmp {
				e.endian = endian
				break
			}
		}
	case types.UnspecifiedOrder:
		break
	}
}

func (e *Evaluator) struc(val *engine.ExprValue) {
	// TODO: handle params here
	oldContext := e.context.Context
	oldEndian := e.endian
	e.context.SetContext(e.context.WithLocalRoot(val))
	e.context.PushStack()
	e.setEndian(val.Type.Struct.Type.Meta.Endian)
	for _, attrVal := range val.Struct.Attrs {
		oldPath := e.path
		e.path = append(e.path, PathItem{
			Name: string(attrVal.Type.Attr.ID),
		})
		e.attr(attrVal)
		e.path = oldPath
	}
	e.context.PopStack()
	e.context.SetContext(oldContext)
	e.endian = oldEndian
}

func (e *Evaluator) attr(val *engine.ExprValue) {
	attr := val.Type.Attr

	rt := attr.Type.FoldEndian(e.endian)

	if attr.If != nil {
		condVal, err := engine.Evaluate(e.context, attr.If)
		if err != nil {
			panic(err)
		}
		if condVal.Type.Kind != engine.BooleanKind {
			panic(fmt.Errorf("if: expected BooleanKind, got %v", condVal.Type.Kind))
		}
		if condVal.Boolean.Value == true {
			return
		}
	}

	if rt.TypeSwitch != nil {
		switchVal, err := engine.Evaluate(e.context, rt.TypeSwitch.SwitchOn)
		if err != nil {
			panic(err)
		}
		for caseEx, typ := range rt.TypeSwitch.Cases {
			caseVal, err := engine.Evaluate(e.context, expr.MustParseExpr(caseEx))
			if err != nil {
				panic(err)
			}
			cmp, err := engine.Compare(switchVal, caseVal, engine.CompareEqual)
			if err != nil {
				panic(err)
			}
			if !cmp {
				continue
			}

			// TODO: should we just pass types.TypeRef to readOne?
			switch typ.Kind {
			case types.User:
				resolved := e.resolveType(typ.User.Name)
				if resolved.Kind != engine.StructKind {
					panic(fmt.Errorf("expression %q yielded unexpected type %s (expected struct)", typ.User.Name, resolved.Kind))
				}
				e.readOne(val, &typ, -1)

			default:
				rt := typ.FoldEndian(e.endian)
				e.readOne(val, &rt, -1)
			}
		}
	} else if attr.Repeat == nil {
		attrVal := e.readOne(val, rt.TypeRef, -1)
		e.context.PutStack(val, attrVal)
	} else {
		attrVal := []*engine.ExprValue{}
		switch repeat := attr.Repeat.(type) {
		case types.RepeatEOS:
			i := 0
			for {
				if eof, err := e.stream.EOF(); err != nil {
					panic(err)
				} else if eof {
					break
				}
				attrVal = append(attrVal, e.readOne(val, rt.TypeRef, i))
				i++
			}

		case types.RepeatExpr:
			countVal, err := engine.Evaluate(e.context, repeat.CountExpr)
			if err != nil {
				panic(err)
			}
			if countVal.Type.Kind != engine.IntegerKind {
				panic(fmt.Errorf("repeat-expr: expected IntegerKind, got %v", countVal.Type.Kind))
			}
			for i := countVal.Integer.Value.Int64(); i > 0; i-- {
				attrVal = append(attrVal, e.readOne(val, rt.TypeRef, int(i)))
			}

		case types.RepeatUntil:
			i := 0
			for {
				untilVal, err := engine.Evaluate(e.context, repeat.UntilExpr)
				if err != nil {
					panic(err)
				}
				if untilVal.Type.Kind != engine.BooleanKind {
					panic(fmt.Errorf("repeat-until: expected BooleanKind, got %v", untilVal.Type.Kind))
				}
				if untilVal.Boolean.Value == true {
					break
				}
				attrVal = append(attrVal, e.readOne(val, rt.TypeRef, i))
				i++
			}
		}
		e.context.PutStack(val, engine.NewArrayLiteralValue(engine.NewArrayType(val.Type, nil), attrVal))
	}
}

func (e *Evaluator) valueOf(value any) *engine.ExprValue {
	switch t := value.(type) {
	case int:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(t)))
	case int8:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(t)))
	case int16:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(t)))
	case int32:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(t)))
	case int64:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(t)))
	case uint:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(t)))
	case uint8:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(t)))
	case uint16:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(t)))
	case uint32:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(t)))
	case uint64:
		return engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(t)))
	case float32:
		return engine.NewFloatLiteralValue(big.NewFloat(float64(t)))
	case float64:
		return engine.NewFloatLiteralValue(big.NewFloat(float64(t)))
	case []byte:
		return engine.NewByteArrayLiteralValue(t)
	case string:
		value = engine.NewStringLiteralValue(t)
	}
	return nil
}

func (e *Evaluator) readOne(attr *engine.ExprValue, n *types.TypeRef, index int) *engine.ExprValue {
	var value any
	startOffset, err := e.stream.Seek(0, io.SeekCurrent)
	if err != nil {
		panic(fmt.Errorf("getting initial offset of span: %w", err))
	}
	oldPath := e.path
	if index >= 0 {
		item := PathItem{
			Name:  e.path[len(e.path)-1].Name,
			Index: new(int),
		}
		*item.Index = index
		e.path = append(e.path[:len(e.path)-1], item)
	}
	path := e.path
	defer func() {
		e.path = oldPath
		endOffset, err := e.stream.Seek(0, io.SeekCurrent)
		if err != nil {
			panic(fmt.Errorf("getting end offset of span: %w", err))
		}
		if value == nil {
			return
		}
		annotation := Annotation{
			Range: Range{
				StartIndex: uint64(startOffset),
				EndIndex:   uint64(endOffset),
			},
			Label: Label{
				Attr:  path,
				Value: value,
			},
		}
		e.annotations = append(e.annotations, annotation)
	}()
	switch n.Kind {
	case types.UntypedInt:
		panic("untyped number")
	case types.U2, types.U4, types.U8,
		types.S2, types.S4, types.S8,
		types.F4, types.F8:
		panic("undecided endianness")
	case types.U1:
		u1, err := e.stream.ReadU1()
		if err != nil {
			panic(err)
		}
		value = u1
	case types.U2le:
		u2, err := e.stream.ReadU2le()
		if err != nil {
			panic(err)
		}
		value = u2
	case types.U2be:
		u2, err := e.stream.ReadU2be()
		if err != nil {
			panic(err)
		}
		value = u2
	case types.U4le:
		u4, err := e.stream.ReadU4le()
		if err != nil {
			panic(err)
		}
		value = u4
	case types.U4be:
		u4, err := e.stream.ReadU4be()
		if err != nil {
			panic(err)
		}
		value = u4
	case types.U8le:
		u8, err := e.stream.ReadU8le()
		if err != nil {
			panic(err)
		}
		value = u8
	case types.U8be:
		u8, err := e.stream.ReadU8be()
		if err != nil {
			panic(err)
		}
		value = u8
	case types.S1:
		s1, err := e.stream.ReadS1()
		if err != nil {
			panic(err)
		}
		value = s1
	case types.S2le:
		s2, err := e.stream.ReadS2le()
		if err != nil {
			panic(err)
		}
		value = s2
	case types.S2be:
		s2, err := e.stream.ReadS2be()
		if err != nil {
			panic(err)
		}
		value = s2
	case types.S4le:
		s4, err := e.stream.ReadS4le()
		if err != nil {
			panic(err)
		}
		value = s4
	case types.S4be:
		s4, err := e.stream.ReadS4be()
		if err != nil {
			panic(err)
		}
		value = s4
	case types.S8le:
		s8, err := e.stream.ReadS8le()
		if err != nil {
			panic(err)
		}
		value = s8
	case types.S8be:
		s8, err := e.stream.ReadS8be()
		if err != nil {
			panic(err)
		}
		value = s8
	case types.Bits:
		panic("not implemented yet: bits")
	case types.F4le:
		f4, err := e.stream.ReadF4le()
		if err != nil {
			panic(err)
		}
		value = f4
	case types.F4be:
		f4, err := e.stream.ReadF4be()
		if err != nil {
			panic(err)
		}
		value = f4
	case types.F8le:
		f8, err := e.stream.ReadF8le()
		if err != nil {
			panic(err)
		}
		value = f8
	case types.F8be:
		f8, err := e.stream.ReadF8be()
		if err != nil {
			panic(err)
		}
		value = f8
	case types.Bytes:
		var data []byte
		if n.Bytes.Size != nil {
			sizeVal, err := engine.Evaluate(e.context, n.Bytes.Size)
			if err != nil {
				panic(err)
			}
			if sizeVal.Type.Kind != engine.IntegerKind {
				panic(fmt.Errorf("size: expected IntegerKind, got %v", sizeVal.Type.Kind))
			}
			data, err = e.stream.ReadBytes(int(sizeVal.Integer.Value.Int64()))
			if err != nil {
				panic(err)
			}
			value = data
		} else if n.Bytes.SizeEOS {
			data, err = e.stream.ReadBytesFull()
			if err != nil {
				panic(err)
			}
			value = data
		} else {
			panic("not implemented yet: bytes")
		}
		if attr.Type.Attr.Contents != nil {
			if !bytes.Equal(data, attr.Type.Attr.Contents) {
				panic(NewValidationNotEqualError(attr.Type.Attr.Contents, data, e.stream, "")) // TODO: set srcPath
			}
		}
	case types.String:
		if n.String.SizeEOS {
			str, err := e.stream.ReadStrEOS(n.String.Encoding)
			if err != nil {
				panic(err)
			}
			value = str
		} else if n.String.Size != nil {
			sizeVal, err := engine.Evaluate(e.context, n.String.Size)
			if err != nil {
				panic(err)
			}
			if sizeVal.Type.Kind != engine.IntegerKind {
				panic(fmt.Errorf("size: expected IntegerKind, got %v", sizeVal.Type.Kind))
			}
			if n.String.Terminator == -1 {
				bytes, err := e.stream.ReadBytes(int(sizeVal.Integer.Value.Int64()))
				if err != nil {
					panic(err)
				}
				value = bytes
			} else {
				bytes, err := e.stream.ReadBytesPadTerm(int(sizeVal.Integer.Value.Int64()), byte(n.String.Terminator), byte(n.String.Terminator), n.String.Include)
				if err != nil {
					panic(err)
				}
				value = bytes
			}
		} else {
			if n.String.Terminator == -1 {
				panic("undecidable condition")
			}
			bytes, err := e.stream.ReadBytesTerm(byte(n.String.Terminator), n.String.Include, n.String.Consume, n.String.EosError)
			if err != nil {
				panic(err)
			}
			value = bytes
		}
	case types.User:
		resolved := e.resolveType(n.User.Name)
		if resolved.Kind != engine.StructKind {
			panic(fmt.Errorf("expression %q yielded unexpected type %s (expected struct)", n.User.Name, resolved.Kind))
		}
		struc := engine.NewStructValueSymbol(resolved, attr.Parent)
		oldStream := e.stream
		oldContext := e.context.Context
		// TODO: is this right? Check upstream. Documentation unclear.
		if n.User.Size != nil {
			sizeVal, err := engine.Evaluate(e.context, n.User.Size)
			if err != nil {
				panic(err)
			}
			if sizeVal.Type.Kind != engine.IntegerKind {
				panic(fmt.Errorf("expression %q yielded unexpected type %s (expected integer)", n.User.Name, sizeVal.Type.Kind))
			}
			size := sizeVal.Integer.Value.Int64()
			offset, err := e.stream.ReadSeeker.Seek(0, io.SeekCurrent)
			if err != nil {
				panic(err)
			}
			if _, err := e.stream.ReadSeeker.Seek(size, io.SeekCurrent); err != nil {
				panic(err)
			}
			e.stream = NewSubStream(e.stream, offset, size)
			e.context.SetContext(e.context.Context.WithStream(engine.NewRuntimeStreamValue(e.stream)))
		}
		e.struc(struc)
		e.stream = oldStream
		e.context.SetContext(oldContext)
		return struc
	default:
		panic("unexpected typekind: " + n.Kind.String())
	}
	return e.valueOf(value)
}

func (e *Evaluator) resolveType(ex string) *engine.ExprType {
	typ := engine.ResultTypeOfExpr(e.context.Context, expr.MustParseExpr(ex)).Type()
	if typ == nil {
		panic(fmt.Errorf("unresolved type: %s", ex))
	}
	return typ
}
