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
	if s.Meta.Endian.Kind != types.UnspecifiedOrder {
		e.endian = s.Meta.Endian.Kind
	}

	// Pivot stack to new root
	root := engine.NewStructValueSymbol(engine.NewStructTypeSymbol(s, nil), nil)
	e.context.AddGlobalType(string(s.ID), root.Type)
	e.context.AddModuleType(string(s.ID), root.Type)
	oldContext := e.context.Context
	e.context.SetContext(e.context.WithModuleRoot(root).WithLocalRoot(root))

	e.struc(root)

	// Pivot back to old root
	e.context.SetContext(oldContext)
}

func (e *Evaluator) struc(val *engine.ExprValue) {
	// TODO: handle params here
	oldContext := e.context.Context
	e.context.SetContext(e.context.WithLocalRoot(val))
	e.context.PushStack()
	for _, attrVal := range val.Struct.Attrs {
		e.attr(attrVal)
	}
	e.context.PopStack()
	e.context.SetContext(oldContext)
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
				e.readOne(val, &typ, 0)

			default:
				rt := typ.FoldEndian(e.endian)
				e.readOne(val, &rt, 0)
			}
		}
	} else if attr.Repeat == nil {
		attrVal := e.readOne(val, rt.TypeRef, 0)
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

func (e *Evaluator) readOne(attr *engine.ExprValue, n *types.TypeRef, index int) *engine.ExprValue {
	var value *engine.ExprValue
	startOffset, err := e.stream.Seek(0, io.SeekCurrent)
	if err != nil {
		panic(fmt.Errorf("getting initial offset of span: %w", err))
	}
	defer func() {
		endOffset, err := e.stream.Seek(0, io.SeekCurrent)
		if err != nil {
			panic(fmt.Errorf("getting end offset of span: %w", err))
		}
		annotation := Annotation{
			Range: Range{
				StartIndex: uint64(startOffset),
				EndIndex:   uint64(endOffset),
			},
			Label: Label{
				Attr:  attr,
				Value: value,
				Index: index,
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
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(u1)))
	case types.U2le:
		u2, err := e.stream.ReadU2le()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(u2)))
	case types.U2be:
		u2, err := e.stream.ReadU2be()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(u2)))
	case types.U4le:
		u4, err := e.stream.ReadU4le()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(u4)))
	case types.U4be:
		u4, err := e.stream.ReadU4be()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(uint64(u4)))
	case types.U8le:
		u8, err := e.stream.ReadU8le()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(u8))
	case types.U8be:
		u8, err := e.stream.ReadU8be()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetUint64(u8))
	case types.S1:
		s1, err := e.stream.ReadS1()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(s1)))
	case types.S2le:
		s2, err := e.stream.ReadS2le()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(s2)))
	case types.S2be:
		s2, err := e.stream.ReadS2be()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(s2)))
	case types.S4le:
		s4, err := e.stream.ReadS4le()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(s4)))
	case types.S4be:
		s4, err := e.stream.ReadS4be()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(int64(s4)))
	case types.S8le:
		s8, err := e.stream.ReadS8le()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(s8))
	case types.S8be:
		s8, err := e.stream.ReadS8be()
		if err != nil {
			panic(err)
		}
		value = engine.NewIntegerLiteralValue(big.NewInt(0).SetInt64(s8))
	case types.Bits:
		panic("not implemented yet: bits")
	case types.F4le:
		f4, err := e.stream.ReadF4le()
		if err != nil {
			panic(err)
		}
		value = engine.NewFloatLiteralValue(big.NewFloat(float64(f4)))
	case types.F4be:
		f4, err := e.stream.ReadF4be()
		if err != nil {
			panic(err)
		}
		value = engine.NewFloatLiteralValue(big.NewFloat(float64(f4)))
	case types.F8le:
		f8, err := e.stream.ReadF8le()
		if err != nil {
			panic(err)
		}
		value = engine.NewFloatLiteralValue(big.NewFloat(f8))
	case types.F8be:
		f8, err := e.stream.ReadF8be()
		if err != nil {
			panic(err)
		}
		value = engine.NewFloatLiteralValue(big.NewFloat(f8))
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
			value = engine.NewByteArrayLiteralValue(data)
		} else if n.Bytes.SizeEOS {
			data, err = e.stream.ReadBytesFull()
			if err != nil {
				panic(err)
			}
			value = engine.NewByteArrayLiteralValue(data)
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
			value = engine.NewStringLiteralValue(str)
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
				value = engine.NewStringLiteralValue(string(bytes))
			} else {
				bytes, err := e.stream.ReadBytesPadTerm(int(sizeVal.Integer.Value.Int64()), byte(n.String.Terminator), byte(n.String.Terminator), n.String.Include)
				if err != nil {
					panic(err)
				}
				value = engine.NewStringLiteralValue(string(bytes))
			}
		} else {
			if n.String.Terminator == -1 {
				panic("undecidable condition")
			}
			bytes, err := e.stream.ReadBytesTerm(byte(n.String.Terminator), n.String.Include, n.String.Consume, n.String.EosError)
			if err != nil {
				panic(err)
			}
			value = engine.NewByteArrayLiteralValue(bytes)
		}
	case types.User:
		resolved := e.resolveType(n.User.Name)
		if resolved.Kind != engine.StructKind {
			panic(fmt.Errorf("expression %q yielded unexpected type %s (expected struct)", n.User.Name, resolved.Kind))
		}
		e.struc(engine.NewStructValueSymbol(resolved, attr.Parent))
	default:
		panic("unexpected typekind: " + n.Kind.String())
	}
	return value
}

func (e *Evaluator) resolveType(ex string) *engine.ExprType {
	typ := engine.ResultTypeOfExpr(e.context.Context, expr.MustParseExpr(ex)).Type()
	if typ == nil {
		panic(fmt.Errorf("unresolved type: %s", ex))
	}
	return typ
}
