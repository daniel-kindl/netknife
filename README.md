# netknife

Netknife is a Linux network diagnostics toolkit. Diagnostic commands are in development.

Build and test with Go:

```sh
go test ./...
go build ./cmd/netknife
```

Run `netknife --help` for usage and `netknife version` for the version. Use `netknife version --json` for JSON output. Builds can set the version with `-ldflags '-X main.version=1.0.0'`; the default is `dev`.

Successful output goes to standard output. Errors go to standard error. The exit code is 0 for success, 1 for an execution or output error, and 2 for invalid arguments. An interrupt or termination signal cancels the command.

Diagnostic commands will support text output and `--json`. JSON results use an object with `command` and `data` fields, followed by a newline. The `data` value contains the command's result. Errors do not produce a JSON result.

The toolkit targets Linux on AMD64 and ARM64.
