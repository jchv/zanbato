package main

import (
	"flag"
	"log"
	"os"

	"github.com/jchv/zanbato/kaitai/eval"
	"github.com/jchv/zanbato/kaitai/resolve"
)

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
	evaluator := eval.NewEvaluator(resolver, stream)
	annotations := evaluator.Evaluate(basename, struc)
	log.Printf("%#v\n", annotations)
}
