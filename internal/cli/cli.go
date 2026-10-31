// Package cli runs commands and renders their results without process-global state.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const DEFAULT_TIMEOUT = 10 * time.Second

// Command keeps execution separate from text rendering. Execute must honor ctx.
// Execute receives positional arguments after the common options are removed.
type Command struct {
	Name    string
	Summary string
	Execute func(ctx context.Context, args []string) (any, error)
	Text    func(out io.Writer, data any) error
}

type options struct {
	json       bool
	help       bool
	timeout    time.Duration
	positional []string
}

func parse(args []string) (options, error) {
	parsed := options{timeout: DEFAULT_TIMEOUT}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			parsed.positional = append(parsed.positional, args[i+1:]...)
			break
		}
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "--json":
			parsed.json = true
			if hasValue {
				var err error
				parsed.json, err = strconv.ParseBool(value)
				if err != nil {
					return parsed, fmt.Errorf("invalid --json value %q", value)
				}
			}
		case "--help", "-h":
			if hasValue {
				return parsed, fmt.Errorf("option %q takes no value", name)
			}
			parsed.help = true
		case "--version", "-v":
			if hasValue {
				return parsed, fmt.Errorf("option %q takes no value", name)
			}
			parsed.positional = append(parsed.positional, "version")
		case "--timeout":
			if !hasValue {
				i++
				if i == len(args) {
					return parsed, errors.New("--timeout requires a duration")
				}
				value = args[i]
			}
			duration, err := time.ParseDuration(value)
			if err != nil {
				return parsed, fmt.Errorf("invalid timeout %q", value)
			}
			if duration <= 0 {
				return parsed, errors.New("timeout must be greater than zero")
			}
			parsed.timeout = duration
		default:
			if strings.HasPrefix(arg, "-") {
				return parsed, fmt.Errorf("unknown option %q", arg)
			}
			parsed.positional = append(parsed.positional, arg)
		}
	}
	return parsed, nil
}

// Run returns an exit status. It writes results to out and errors to errOut.
func Run(ctx context.Context, args []string, out, errOut io.Writer, version string, commands []Command) int {
	fail := func(err error, code int) int { fmt.Fprintln(errOut, "netknife:", err); return code }
	parsed, err := parse(args)
	if err != nil {
		return fail(err, 2)
	}
	versionCommand := Command{Name: "version", Summary: "Show the version", Execute: func(context.Context, []string) (any, error) {
		return struct {
			Version string `json:"version"`
		}{version}, nil
	}, Text: func(out io.Writer, _ any) error { _, err := fmt.Fprintln(out, version); return err }}
	available := append([]Command{versionCommand}, commands...)
	name := "help"
	var positional []string
	if len(parsed.positional) > 0 {
		name = parsed.positional[0]
		positional = parsed.positional[1:]
	}
	if name == "help" || parsed.help {
		if name == "help" && len(positional) > 1 {
			return fail(errors.New("help takes at most one command"), 2)
		}
		target := name
		if name == "help" {
			target = ""
			if len(positional) == 1 {
				target = positional[0]
			}
		}
		if target != "" && target != "help" {
			found := false
			for _, command := range available {
				if command.Name == target {
					found = true
				}
			}
			if !found {
				return fail(fmt.Errorf("unknown command %q", target), 2)
			}
		}
		var text strings.Builder
		if target != "" && target != "help" {
			fmt.Fprintf(&text, "Usage: netknife %s [options]\n", target)
		} else {
			fmt.Fprintln(&text, "Usage: netknife <command> [options]\n\nCommands:\n  help       Show help")
			for _, command := range available {
				fmt.Fprintf(&text, "  %-10s %s\n", command.Name, command.Summary)
			}
		}
		fmt.Fprint(&text, "\nOptions:\n  -h, --help       Show help\n  -v, --version    Show the version\n  --json          Write a JSON result\n  --timeout <d>   Execution timeout (default 10s)\n")
		if _, err := io.WriteString(out, text.String()); err != nil {
			return fail(err, 1)
		}
		return 0
	}
	var selected *Command
	for i := range available {
		if available[i].Name == name {
			selected = &available[i]
			break
		}
	}
	if selected == nil {
		return fail(fmt.Errorf("unknown command %q", name), 2)
	}
	if name == "version" && len(positional) > 0 {
		return fail(errors.New("version takes no arguments"), 2)
	}
	executionCtx, cancel := context.WithTimeout(ctx, parsed.timeout)
	defer cancel()
	checkContext := func() int {
		if errors.Is(executionCtx.Err(), context.Canceled) {
			return fail(errors.New("cancelled"), 130)
		}
		return fail(errors.New("timeout exceeded"), 1)
	}
	if executionCtx.Err() != nil {
		return checkContext()
	}
	data, err := selected.Execute(executionCtx, positional)
	if executionCtx.Err() != nil {
		return checkContext()
	}
	if err != nil {
		return fail(err, 1)
	}
	if parsed.json {
		encoded, err := json.Marshal(struct {
			Command string `json:"command"`
			Data    any    `json:"data"`
		}{name, data})
		if err != nil {
			return fail(fmt.Errorf("encode result: %w", err), 1)
		}
		encoded = append(encoded, '\n')
		if _, err := out.Write(encoded); err != nil {
			return fail(err, 1)
		}
	} else {
		if selected.Text == nil {
			return fail(errors.New("command has no text renderer"), 1)
		}
		if err := selected.Text(out, data); err != nil {
			return fail(err, 1)
		}
	}
	return 0
}
