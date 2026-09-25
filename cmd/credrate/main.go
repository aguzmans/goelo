// credrate applies one stateless rating request from a JSON file or stdin.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aguzmans/goelo/core"
)

// version is set by GoReleaser for tagged builds.
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "credrate:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		fmt.Fprintln(stdout, version)
		return nil
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(stdout, "Usage: credrate rate [request.json]\n       credrate version\n\nReads one JSON rating request from request.json or stdin and writes the result to stdout.")
		return nil
	}
	if args[0] != "rate" || len(args) > 2 {
		return errors.New("usage: credrate rate [request.json]")
	}
	input := stdin
	if len(args) == 2 {
		f, err := os.Open(args[1])
		if err != nil {
			return fmt.Errorf("open request: %w", err)
		}
		defer f.Close()
		input = f
	}
	dec := json.NewDecoder(input)
	dec.DisallowUnknownFields()
	var req core.Request
	if err := dec.Decode(&req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request must contain exactly one JSON value")
		}
		return fmt.Errorf("decode trailing input: %w", err)
	}
	result, err := core.Rate(req)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	return nil
}
