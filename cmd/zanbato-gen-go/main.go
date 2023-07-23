package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter/golang"
)

func main() {
	pkg := flag.String("pkg", "", "Go package path to use")
	out := flag.String("out", "", "File system output path to use")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalln("Wrong number of arguments; pass your root .ksy path.")
	}
	rootname := flag.Arg(0)
	resolver := func(from, to string) (string, *kaitai.Struct) {
		basename := to
		if from != "" {
			basename = path.Join(path.Dir(from), to)
		}
		candidates := []string{basename + ".ksy", basename}
		for _, name := range candidates {
			file, err := os.Open(name)
			if err != nil {
				continue
			}
			defer file.Close()
			struc, err := kaitai.ParseStruct(file)
			if err != nil {
				panic(fmt.Errorf("error loading %q: %v", name, err))
			}
			return basename, struc
		}
		panic(fmt.Errorf("failed to load %s from %s (checked %v)", to, from, candidates))
	}
	emitter := golang.NewEmitter(*pkg, resolver)
	artifacts := emitter.Emit(resolver("", rootname))
	for _, artifact := range artifacts {
		outname := filepath.Join(*out, artifact.Filename)
		file, err := os.Create(outname)
		if err != nil {
			log.Fatalf("Error creating %s: %w", outname, err)
		}
		_, err = file.Write(artifact.Body)
		if err != nil {
			log.Fatalf("Error writing %s: %w", outname, err)
		}
	}
}
