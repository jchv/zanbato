package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/jchv/zanbato/kaitai/emitter/wireshark"
	"github.com/jchv/zanbato/kaitai/resolve"
)

func main() {
	out := flag.String("out", "", "File system output path to use")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalln("Wrong number of arguments; pass your root .ksy path.")
	}
	rootname := flag.Arg(0)
	resolver := resolve.NewOSResolver()
	emitter := wireshark.NewEmitter(*out, resolver)
	basename, struc, err := resolver.Resolve("", rootname)
	if err != nil {
		log.Fatalf("error resolving root struct: %v", err)
	}
	os.MkdirAll(*out, os.ModeDir|0o755)
	artifacts := emitter.Emit(basename, struc)
	for _, artifact := range artifacts {
		outname := filepath.Join(*out, artifact.Filename)
		file, err := os.Create(outname)
		if err != nil {
			log.Fatalf("Error creating %s: %v", outname, err)
		}
		_, err = file.Write(artifact.Body)
		if err != nil {
			log.Fatalf("Error writing %s: %v", outname, err)
		}
	}
}
