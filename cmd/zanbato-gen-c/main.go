package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter/c"
	"github.com/jchv/zanbato/kaitai/resolve"
)

func main() {
	var compat kaitai.Compatibility

	out := flag.String("out", "", "Output directory")
	debug := flag.Bool("debug", false, "Enable debug features in generated code")
	flag.Var(&compat, "compat", "Compatibility mode: native (default) or 0.11")
	importPaths := resolve.RegisterImportPathsFlag(flag.CommandLine)
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalln("Wrong number of arguments; pass your root .ksy path.")
	}
	rootname := flag.Arg(0)
	resolver := resolve.NewOSResolverWithPaths(*importPaths)
	emitter := c.NewEmitter(resolver)
	emitter.SetDebug(*debug)
	emitter.SetCompat(compat)
	basename, struc, err := resolver.Resolve("", rootname)
	if err != nil {
		log.Fatalf("Error resolving root struct: %v", err)
	}
	if err := os.MkdirAll(*out, os.ModeDir|0o755); err != nil {
		log.Fatalf("error creating output directory: %v", err)
	}
	artifacts := emitter.Emit(basename, struc)
	for _, artifact := range artifacts {
		path := filepath.Join(*out, artifact.Filename)
		err := os.WriteFile(path, artifact.Body, 0o644)
		if err != nil {
			log.Fatalf("Error writing %s: %v", path, err)
		}
	}
}
