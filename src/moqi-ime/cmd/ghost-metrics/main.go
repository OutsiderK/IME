package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/gaboolic/moqi-ime/internal/ghosttelemetry"
)

func main() {
	inputPath := flag.String("input", "-", "JSONL event file, or - for stdin")
	clearInput := flag.Bool("clear", false, "remove the exact input file and exit")
	flag.Parse()
	if *clearInput {
		if *inputPath == "-" {
			fmt.Fprintln(os.Stderr, "-clear requires an explicit -input file")
			os.Exit(1)
		}
		if err := os.Remove(*inputPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("removed %s\n", *inputPath)
		return
	}
	input, closeInput, err := openInput(*inputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer closeInput()
	metrics, err := ghosttelemetry.Aggregate(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoded, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open input: %w", err)
	}
	return f, func() { _ = f.Close() }, nil
}
