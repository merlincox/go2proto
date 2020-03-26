package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/merlincox/go2proto/pkg/parser"
	"github.com/merlincox/go2proto/pkg/protobuf"
)

func main() {

	inputPathsString := flag.String("p", "", `Comma-separated paths of package directories to analyse for structs. Relative paths ("./example/in") are allowed.`)
	structFilter := flag.String("f", "", "Filter by struct names.")
	outputPath := flag.String("o", "./output.proto", "Protobuf output file path. The output directory must exist.")
	useTags := flag.Bool("t", false, "Add import tagger/tagger.proto and write tag extensions if any of the structs are tagged.")

	flag.Parse()

	if len(*inputPathsString) == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	inputPaths := strings.Split(*inputPathsString, ",")

	outputFolder := filepath.Dir(*outputPath)

	//ensure the output directory exists and is a directory
	info, err := os.Stat(outputFolder)

	if os.IsNotExist(err) {
		log.Fatalf("Output folder %s does not exist", outputFolder)
	}

	if info.Mode().IsRegular() {
		log.Fatalf("%s is not a directory", outputFolder)
	}

	absOutputFilePath, err := filepath.Abs(*outputPath)

	if err != nil {
		log.Fatalf("Error getting absolute output path: %s", err)
	}

	msgs, err := parser.GetMessages(inputPaths, *structFilter)

	if err != nil {
		log.Fatalf("Error getting messages: %s", err)
	}

	if len(msgs) == 0 {
		log.Fatalf("No messages were found")
	}

	fout, err := os.Create(absOutputFilePath)

	if err != nil {
		log.Fatalf("Unable to create file %s : %s", absOutputFilePath, err)
	}

	defer fout.Close()

	if err = protobuf.WriteOutput(fout, msgs, *useTags); err != nil {
		log.Fatalf("Error writing output: %s", err)
	}

	log.Printf("Output file written to %s\n", absOutputFilePath)
}
