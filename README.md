# netknife

Netknife is a Linux network diagnostics toolkit for AMD64 and ARM64. The `info` command shows local network interfaces. The `ping` command checks ICMP connectivity. DNS and route diagnostics are being developed in separate issues.

Build and test with Go 1.26 or later:

```sh
go test ./...
go build -o netknife ./cmd/netknife
./netknife --help
./netknife version
./netknife version --json
./netknife info
./netknife info --json
./netknife ping 127.0.0.1 --count 1
./netknife ping example.com --count 5 --timeout 8s --json
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

The executable registers help, version, info, and ping. The internal command runner accepts context-aware diagnostic functions and separate text formatters. The `dns` and `trace` implementations are planned separately.

Command-specific options belong to each command. `Command.New` registers them on a fresh standard-library `flag.FlagSet` and returns an execution function that uses those invocation-local values. Command options must appear after the command name. The shared parser consumes each option value according to its registered type, so values that resemble common flags remain values. Common option names are reserved. Unknown options fail before execution. Command help displays the registered options. `Command.Arguments` describes positional values in the usage line. Diagnostic issues can register flags without changing the common parser.

## Local network information

`netknife info` reads the interfaces in the current network namespace. It needs no external tools, root access, or network connection. It includes loopback and down interfaces. It accepts no positional arguments or command-specific flags. Common `--json` and `--timeout` flags apply.

Each interface has these JSON fields:

| Field | Meaning |
| --- | --- |
| `index` | OS interface index |
| `name` | OS interface name |
| `mtu` | Maximum transmission unit in bytes |
| `flags` | Go interface flags, such as `up`, `loopback`, or `broadcast` |
| `hardware_address` | Hardware address, or an empty string when absent |
| `addresses` | Assigned unicast IPv4 and IPv6 address strings with prefix lengths |

The result is `{"command":"info","data":{"interfaces":[...]}}`. Interfaces are sorted by index, then name. Addresses are sorted lexically. Empty collections are `[]`. No interfaces is a successful result. Text output states `No network interfaces found.` in that case. Missing flags, hardware addresses, or addresses appear as `(none)` in text.

If enumeration or an address read fails, the command exits with code 1 and names the failed operation on stderr. It emits no partial result. A configuration change during collection can cause an address read to fail. The result is an observation across several OS calls, not an atomic snapshot.

Cancellation and timeout checks run before and after each synchronous OS read. Go's interface APIs do not accept a context, so an OS read must return before cancellation can finish. The command does not inspect routes, DNS configuration, or link history.

## Connectivity checks

`netknife ping <host>` sends ICMP echo probes through Linux iputils `ping` from `PATH`.
Install `iputils-ping` on Debian or Ubuntu, or `iputils` on Fedora or Arch Linux.
Other implementations, such as BusyBox `ping`, are not supported.
Netknife starts no shell and does not request elevated privileges.
The installed tool needs permission to send ICMP probes.
If a permission error occurs, ask the system administrator to check the iputils installation and the system's ICMP permissions.

```sh
netknife ping 1.1.1.1 --count 5
netknife ping example.com --timeout 8s --json
netknife ping ::1 --count 1
netknife help ping
```

The host must be one IPv4 address, IPv6 address, or ASCII DNS name.
Netknife accepts IPv6 interface zones, such as `fe80::1%eth0`.
Use ASCII punycode for international domain names.
Netknife rejects URLs, whitespace, shell syntax, and names that start with a hyphen.
Each DNS label must contain 1 through 63 letters, digits, or hyphens, with no hyphen at either end.
The DNS name can have a trailing dot and at most 253 characters without that dot.
Zone names accept ASCII letters, digits, underscores, hyphens, and dots.

`--count <n>` selects 1 through 2147483647 probes. The default is `4`.
The common `--timeout` limits the whole run, including name resolution. Its default is `10s`.
Choose a longer timeout for large counts. The tool normally sends one probe per second.
Netknife uses iputils `-W 1` to limit the response wait when no replies arrive.
After the last probe, iputils waits for two round-trip times if it received replies.
If the whole-command timeout expires, netknife reports a timeout and emits no result.

Text output shows the host, transmitted and received packet totals, packet loss, and round-trip times.
JSON uses this schema:

```json
{"command":"ping","data":{"host":"example.com","transmitted":4,"received":3,"loss_percent":25,"rtt":{"min_ms":1,"avg_ms":2,"max_ms":3,"mdev_ms":0.5}}}
```

`host` is the requested host, not the resolved address.
`loss_percent` is the percentage of transmitted probes without replies.
The `rtt` fields report round-trip times in milliseconds.
`min_ms`, `avg_ms`, and `max_ms` are the minimum, average, and maximum times.
`mdev_ms` is the population standard deviation of those times.
Duplicate replies do not increase `received` but can affect round-trip times.
A completed run with at least one reply succeeds, including partial packet loss.
No replies is an execution failure. The target may be unreachable, or a firewall may block ICMP.
ICMP replies alone do not prove that an application service is available.

Invalid arguments exit with code 2 before tool execution.
Missing tools, permission failures, resolution failures, unsupported output, and no replies exit with code 1.
These failures leave stdout empty and write a diagnostic to stderr, including with `--json`.
Cancellation stops the subprocess and exits with code 130. Timeout stops it and exits with code 1.
Netknife retains at most 64 KiB each from the tool's standard output and standard error.
If excessive tool output prevents summary parsing, the command fails instead of returning incomplete statistics.

The deterministic tests use local executable fixtures and require no network access or ICMP permissions.
Live loopback checks can test the installed iputils tool. Public-host checks depend on local network and firewall configuration.
