# netknife

Netknife is a Linux network diagnostics toolkit for AMD64 and ARM64. The CLI foundation is available. Network diagnostics are being developed in separate issues.

Build and test with Go 1.26 or later:

```sh
go test ./...
go build -o netknife ./cmd/netknife
./netknife --help
./netknife version
./netknife version --json
```

The default version is `dev`. Set a release version at build time:

```sh
go build -ldflags '-X main.version=1.0.0' -o netknife ./cmd/netknife
```

## Command and flag contract

`help`, `--help`, and `-h` show help. With no arguments, netknife shows help. Use `help version` or `version --help` for command help. `version` shows the build version. `--version` and `-v` are standalone shortcuts. They accept common flags such as `--json`, but cannot be combined with a command. For example, `--version info` is an argument error, even if `info` is registered.

Common flags can appear before or after the command and its arguments:

| Flag | Behavior |
| --- | --- |
| `--json` | Produce a JSON result. `--json=false` selects text. |
| `--timeout 3s` | Limit execution to a positive Go duration. The default is `10s`. `--timeout=3s` is also accepted. |
| `--` | Stop option parsing. All following values are positional arguments. |

For repeated flags, the last value wins. Unknown options and commands are errors. Help remains text even with `--json`. Timeout applies to command execution, not help or output writes. Command functions must honor their context. Interrupt and termination signals cancel that context.

## Output contract

Text is the default. Version text is the version string followed by a newline. Future diagnostic commands define their own readable text and JSON data schemas.

A successful JSON command writes one object followed by a newline to standard output. The envelope contains `command` and `data`. For example, `netknife version --json` writes:

```json
{"command":"version","data":{"version":"dev"}}
```

Command execution returns structured data. A separate formatter produces text. The CLI encodes the data directly for JSON output. It completes JSON encoding before writing, so an encoding error does not emit a partial JSON value.

Errors go to standard error as `netknife: <reason>` followed by a newline. Argument and execution failures leave standard output empty. A failed output write can leave partial output.

| Exit code | Meaning |
| --- | --- |
| `0` | Success, including help |
| `1` | Execution, timeout, encoding, or output failure |
| `2` | Invalid command, option, or arguments |
| `130` | Cancellation |

The executable currently registers only help and version. The internal command runner accepts context-aware diagnostic functions and separate text formatters. The `info`, `ping`, `dns`, and `trace` implementations are outside this change.

Command-specific options belong to each command. `Command.New` registers them on a fresh standard-library `flag.FlagSet` and returns an execution function that uses those invocation-local values. Command options must appear after the command name. The shared parser consumes each option value according to its registered type, so values that resemble common flags remain values. Common option names are reserved. Unknown options fail before execution. Command help displays the registered options. Diagnostic issues can register flags without changing the common parser.
