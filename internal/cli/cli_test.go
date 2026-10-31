package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestCLIContract(t *testing.T) {
	for _, tc := range []struct {
		args            []string
		code            int
		out, diagnostic string
	}{
		{[]string{"version"}, 0, "dev\n", ""},
		{[]string{"-v"}, 0, "dev\n", ""},
		{[]string{"--version"}, 0, "dev\n", ""},
		{[]string{"version", "--json=false"}, 0, "dev\n", ""},
		{[]string{"version", "--json"}, 0, "{\"command\":\"version\",\"data\":{\"version\":\"dev\"}}\n", ""},
		{[]string{"--json", "version"}, 0, "{\"command\":\"version\",\"data\":{\"version\":\"dev\"}}\n", ""},
		{[]string{"missing"}, 2, "", "netknife: unknown command \"missing\"\n"},
		{[]string{"version", "extra"}, 2, "", "netknife: version takes no arguments\n"},
		{[]string{"--timeout=0", "version"}, 2, "", "netknife: timeout must be greater than zero\n"},
		{[]string{"--wat"}, 2, "", "netknife: unknown option \"--wat\"\n"},
	} {
		t.Run(fmt.Sprint(tc.args), func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			code := Run(context.Background(), tc.args, &out, &diagnostic, "dev", nil)
			if code != tc.code || out.String() != tc.out || diagnostic.String() != tc.diagnostic {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
			}
		})
	}
}

type probeResult struct {
	Host string `json:"host"`
}

func TestDiagnosticRendering(t *testing.T) {
	command := Command{Name: "probe", Summary: "Test command", New: withoutFlags(func(ctx context.Context, args []string) (any, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 3*time.Second {
			return nil, errors.New("missing timeout")
		}
		return probeResult{args[0]}, nil
	}), Text: func(out io.Writer, data any) error {
		_, err := fmt.Fprintln(out, "host:", data.(probeResult).Host)
		return err
	}}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"probe", "example.com", "--timeout=3s"}, "host: example.com\n"},
		{[]string{"--json", "probe", "example.com", "--timeout", "3s"}, "{\"command\":\"probe\",\"data\":{\"host\":\"example.com\"}}\n"},
	} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), tc.args, &out, &diagnostic, "dev", []Command{command}); code != 0 || out.String() != tc.want || diagnostic.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}

func TestCancellationDuringExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := Command{Name: "probe", New: withoutFlags(func(ctx context.Context, _ []string) (any, error) { cancel(); <-ctx.Done(); return nil, ctx.Err() })}
	var out, diagnostic bytes.Buffer
	if code := Run(ctx, []string{"probe", "--json"}, &out, &diagnostic, "dev", []Command{command}); code != 130 || out.Len() != 0 || diagnostic.String() != "netknife: cancelled\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
	}
}

func TestHelpAndValidation(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"help"}, {"help", "version"}, {"version", "--help"}} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), args, &out, &diagnostic, "dev", nil); code != 0 || !bytes.Contains(out.Bytes(), []byte("Usage: netknife")) || !bytes.Contains(out.Bytes(), []byte("--timeout <d>")) || diagnostic.Len() != 0 {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, out.String(), diagnostic.String())
		}
	}
	for _, args := range [][]string{{"--timeout"}, {"--timeout=bad", "version"}, {"--json=bad", "version"}, {"help", "missing"}, {"version", "--bad"}, {"--help=yes"}, {"--version=yes"}} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), args, &out, &diagnostic, "dev", nil); code != 2 || out.Len() != 0 || diagnostic.Len() == 0 {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, out.String(), diagnostic.String())
		}
	}
}

func TestExecutionErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		execute func(context.Context, []string) (any, error)
		want    string
		code    int
	}{
		{"failure", context.Background(), func(context.Context, []string) (any, error) { return nil, errors.New("lookup failed") }, "netknife: lookup failed\n", 1},
		{"encoding", context.Background(), func(context.Context, []string) (any, error) { return make(chan int), nil }, "netknife: encode result: json: unsupported type: chan int\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			code := Run(tc.ctx, []string{"probe", "--json"}, &out, &diagnostic, "dev", []Command{{Name: "probe", New: withoutFlags(tc.execute)}})
			if code != tc.code || out.Len() != 0 || diagnostic.String() != tc.want {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
			}
		})
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	var out, diagnostic bytes.Buffer
	if code := Run(ctx, []string{"version"}, &out, &diagnostic, "dev", nil); code != 1 || out.Len() != 0 || diagnostic.String() != "netknife: timeout exceeded\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("output failed") }
func TestWriteFailures(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"version"}, {"version", "--json"}} {
		var diagnostic bytes.Buffer
		if code := Run(context.Background(), args, brokenWriter{}, &diagnostic, "dev", nil); code != 1 || diagnostic.String() != "netknife: output failed\n" {
			t.Fatalf("code=%d stderr=%q", code, diagnostic.String())
		}
	}
}

func TestTimeoutDuringExecution(t *testing.T) {
	started := false
	command := Command{Name: "probe", New: withoutFlags(func(ctx context.Context, _ []string) (any, error) {
		started = true
		<-ctx.Done()
		return nil, ctx.Err()
	})}
	var out, diagnostic bytes.Buffer
	if code := Run(context.Background(), []string{"probe", "--timeout=1s", "--json"}, &out, &diagnostic, "dev", []Command{command}); code != 1 || !started || out.Len() != 0 || diagnostic.String() != "netknife: timeout exceeded\n" {
		t.Fatalf("started=%v code=%d stdout=%q stderr=%q", started, code, out.String(), diagnostic.String())
	}
}

func TestPositionalAndRepeatedOptions(t *testing.T) {
	command := Command{Name: "probe", New: withoutFlags(func(_ context.Context, args []string) (any, error) { return args, nil })}
	var out, diagnostic bytes.Buffer
	code := Run(context.Background(), []string{"--json=false", "probe", "--json", "--", "--host", "example.com"}, &out, &diagnostic, "dev", []Command{command})
	if code != 0 || out.String() != "{\"command\":\"probe\",\"data\":[\"--host\",\"example.com\"]}\n" || diagnostic.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
	}
}

func TestCommandOwnsFlags(t *testing.T) {
	command := Command{Name: "probe", Summary: "Probe a host", New: func(flags *flag.FlagSet) ExecuteFunc {
		count := flags.Int("count", 1, "Number of checks")
		return func(_ context.Context, args []string) (any, error) {
			return struct {
				Count int    `json:"count"`
				Host  string `json:"host"`
			}{*count, args[0]}, nil
		}
	}}
	for _, args := range [][]string{{"probe", "example.com", "--count", "5", "--json"}, {"--json", "probe", "--count=5", "example.com"}} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), args, &out, &diagnostic, "dev", []Command{command}); code != 0 || out.String() != "{\"command\":\"probe\",\"data\":{\"count\":5,\"host\":\"example.com\"}}\n" || diagnostic.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
	var out, diagnostic bytes.Buffer
	if code := Run(context.Background(), []string{"probe", "--count=bad"}, &out, &diagnostic, "dev", []Command{command}); code != 2 || out.Len() != 0 || diagnostic.Len() == 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
	}
}

func TestVersionShortcutIsStandalone(t *testing.T) {
	info := Command{Name: "info", New: withoutFlags(func(context.Context, []string) (any, error) {
		t.Fatal("info executed despite conflicting version shortcut")
		return nil, nil
	})}
	for _, args := range [][]string{{"--version", "info"}, {"info", "--version"}, {"-v", "version"}} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), args, &out, &diagnostic, "dev", []Command{info}); code != 2 || out.Len() != 0 || diagnostic.String() != "netknife: --version and -v must be used without a command\n" {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}

func withoutFlags(execute ExecuteFunc) func(*flag.FlagSet) ExecuteFunc {
	return func(*flag.FlagSet) ExecuteFunc { return execute }
}

func TestCommandHelpAndOptionValues(t *testing.T) {
	command := Command{Name: "probe", New: func(flags *flag.FlagSet) ExecuteFunc {
		label := flags.String("label", "default", "Probe label")
		return func(context.Context, []string) (any, error) { return *label, nil }
	}}
	for _, args := range [][]string{{"help", "probe"}, {"probe", "--help"}} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), args, &out, &diagnostic, "dev", []Command{command}); code != 0 || !bytes.Contains(out.Bytes(), []byte("Probe label")) || diagnostic.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"probe", "--label", "--json", "--json"}, "{\"command\":\"probe\",\"data\":\"--json\"}\n"},
		{[]string{"probe", "--json"}, "{\"command\":\"probe\",\"data\":\"default\"}\n"},
	} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), tc.args, &out, &diagnostic, "dev", []Command{command}); code != 0 || out.String() != tc.want || diagnostic.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}

func TestCommandBooleanAndMissingValue(t *testing.T) {
	command := Command{Name: "probe", New: func(flags *flag.FlagSet) ExecuteFunc {
		enabled := flags.Bool("enabled", false, "Enable probe")
		flags.Int("count", 1, "Number of checks")
		return func(context.Context, []string) (any, error) { return *enabled, nil }
	}}
	for _, tc := range []struct {
		args            []string
		code            int
		out, diagnostic string
	}{
		{[]string{"probe", "--enabled", "--json"}, 0, "{\"command\":\"probe\",\"data\":true}\n", ""},
		{[]string{"probe", "--enabled=false", "--json"}, 0, "{\"command\":\"probe\",\"data\":false}\n", ""},
		{[]string{"probe", "--count"}, 2, "", "netknife: --count requires a value\n"},
	} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), tc.args, &out, &diagnostic, "dev", []Command{command}); code != tc.code || out.String() != tc.out || diagnostic.String() != tc.diagnostic {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}
