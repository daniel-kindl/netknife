package main

import (
	"fmt"
	"io"
	"os"
)

func run(out io.Writer) error {
	_, err := fmt.Fprintln(out, "netknife: network diagnostics toolkit")
	return err
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
