package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter/golang"
)

func main() {
	pkg := flag.String("pkg", "defs", "Go package name to use")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalln("Wrong number of arguments; pass your root .ksy path.")
	}
	root := flag.Arg(0)
	file, err := os.Open(root)
	if err != nil {
		log.Fatalf("Opening file %q: %v", root, err)
	}
	struc, err := kaitai.ParseStruct(file)
	if err != nil {
		log.Fatalf("Reading and parsing file %q: %v", root, err)
	}
	emitter := golang.NewEmitter(*pkg)
	artifacts := emitter.Emit(struc)
	for _, artifact := range artifacts {
		fmt.Printf("// %s:\n%s", artifact.Filename, string(artifact.Body))
	}
}
