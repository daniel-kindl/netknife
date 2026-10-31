package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

var version = "dev"

const help = `Usage: netknife <command> [options]

Commands:
  help       Show this help
  version    Show the version

Options:
  -h, --help     Show help
  -v, --version  Show the version

Version options:
  --json         Show the version as JSON
`

type result struct {
	Command string `json:"command"`
	Data    any    `json:"data"`
}

func render(out io.Writer, value result, jsonOutput bool) error {
	if jsonOutput {
		return json.NewEncoder(out).Encode(value)
	}
	_, err := fmt.Fprintln(out, value.Data)
	return err
}

func run(ctx context.Context, args []string, out, errOut io.Writer) int {
	if err := ctx.Err(); err != nil {
		fmt.Fprintln(errOut, "netknife: cancelled")
		return 1
	}
	if len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		if _, err := io.WriteString(out, help); err != nil {
			fmt.Fprintln(errOut, "netknife:", err)
			return 1
		}
		return 0
	}
	if len(args) == 1 && (args[0] == "version" || args[0] == "-v" || args[0] == "--version") || len(args) == 2 && args[0] == "version" && args[1] == "--json" {
		if err := render(out, result{Command: "version", Data: version}, len(args) == 2); err != nil {
			fmt.Fprintln(errOut, "netknife:", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(errOut, "netknife: invalid arguments: %q\n", args)
	fmt.Fprintln(errOut, "Run 'netknife --help' for usage.")
	return 2
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}
