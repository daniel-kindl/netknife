// Package cli runs commands and renders their results without process-global state.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const DEFAULT_TIMEOUT = 10 * time.Second

// ExecuteFunc runs one invocation and returns data for rendering.
// Implementations must honor ctx.
type ExecuteFunc func(ctx context.Context, args []string) (any, error)

// Command creates invocation-local flags and execution state. New registers only
// command-specific flags. It must not register the reserved common option names.
// The returned function receives positional arguments after option parsing.
type Command struct {
	Name    string
	Summary string
	New     func(flags *flag.FlagSet) ExecuteFunc
	Text    func(out io.Writer, data any) error
}

type options struct {
	json       bool
	help       bool
	timeout    time.Duration
	positional []string
	version    bool
	execute    ExecuteFunc
	flags      *flag.FlagSet
}

func parse(args []string, commands []Command) (options, error) {
	parsed := options{timeout: DEFAULT_TIMEOUT}
	addPositional := func(value string) {
		parsed.positional = append(parsed.positional, value)
		if len(parsed.positional) != 1 {
			return
		}
		for _, command := range commands {
			if command.Name == value {
				parsed.flags = flag.NewFlagSet(value, flag.ContinueOnError)
				parsed.flags.SetOutput(io.Discard)
				parsed.execute = command.New(parsed.flags)
				break
			}
		}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			for _, value := range args[i+1:] {
				addPositional(value)
			}
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
			parsed.version = true
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
				optionName := strings.TrimLeft(name, "-")
				if parsed.flags == nil || parsed.flags.Lookup(optionName) == nil {
					return parsed, fmt.Errorf("unknown option %q", arg)
				}
				option := parsed.flags.Lookup(optionName)
				if !hasValue {
					boolean, ok := option.Value.(interface{ IsBoolFlag() bool })
					if ok && boolean.IsBoolFlag() {
						value = "true"
					} else {
						i++
						if i == len(args) {
							return parsed, fmt.Errorf("%s requires a value", name)
						}
						value = args[i]
					}
				}
				if err := parsed.flags.Set(optionName, value); err != nil {
					return parsed, fmt.Errorf("invalid %s value %q: %w", name, value, err)
				}
			} else {
				addPositional(arg)
			}
		}
	}
	if parsed.version {
		if len(parsed.positional) > 0 {
			return parsed, errors.New("--version and -v must be used without a command")
		}
		addPositional("version")
	}

	return parsed, nil
}

// Run returns an exit status. It writes results to out and errors to errOut.
func Run(ctx context.Context, args []string, out, errOut io.Writer, version string, commands []Command) int {
	fail := func(err error, code int) int { fmt.Fprintln(errOut, "netknife:", err); return code }
	versionCommand := Command{Name: "version", Summary: "Show the version", New: func(*flag.FlagSet) ExecuteFunc {
		return func(context.Context, []string) (any, error) {
			return struct {
				Version string `json:"version"`
			}{version}, nil
		}
	}, Text: func(out io.Writer, _ any) error { _, err := fmt.Fprintln(out, version); return err }}
	available := append([]Command{versionCommand}, commands...)
	parsed, err := parse(args, available)
	if err != nil {
		return fail(err, 2)
	}
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
					if parsed.flags == nil {
						parsed.flags = flag.NewFlagSet(target, flag.ContinueOnError)
						command.New(parsed.flags)
					}
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
		fmt.Fprint(&text, "\nOptions:\n  -h, --help       Show help\n  -v, --version    Show the version (standalone)\n  --json          Write a JSON result\n  --timeout <d>   Execution timeout (default 10s)\n")
		if parsed.flags != nil {
			parsed.flags.SetOutput(&text)
			parsed.flags.PrintDefaults()
		}
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
	data, err := parsed.execute(executionCtx, positional)
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
