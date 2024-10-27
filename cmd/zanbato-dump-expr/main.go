package main

import (
	"flag"

	"github.com/davecgh/go-spew/spew"
	"github.com/jchv/zanbato/kaitai/expr"
)

func main() {
	flag.Parse()
	s := spew.NewDefaultConfig()
	s.ContinueOnMethod = true
	for _, arg := range flag.Args() {
		e := expr.MustParseExpr(arg)
		s.Dump(e)
	}
}
