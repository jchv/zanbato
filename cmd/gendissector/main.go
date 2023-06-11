package main

import (
	"flag"
	"log"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/jchv/zanbato/kaitai"
)

func main() {
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
	spew.Dump(struc)
}
