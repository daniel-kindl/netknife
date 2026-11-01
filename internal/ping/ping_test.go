package ping_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/daniel-kindl/netknife/internal/ping"
)

const SUMMARY = "PING example.com (192.0.2.1) 56(84) bytes of data.\n\n--- example.com ping statistics ---\n4 packets transmitted, 3 received, 25% packet loss, time 3004ms\nrtt min/avg/max/mdev = 1.000/2.000/3.000/0.500 ms\n"

func executable(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ping"), []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestRun(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("ARGS_FILE", argsFile)
	t.Setenv("SUMMARY", SUMMARY)
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	executable(t, `printf '%s\n' "$LC_ALL" "$@" > "$ARGS_FILE"
printf '%s' "$SUMMARY"
`)
	got, err := ping.Run(context.Background(), "example.com", 4)
	want := ping.Result{Host: "example.com", Transmitted: 4, Received: 3, LossPercent: 25, RTT: ping.RTT{MinMS: 1, AvgMS: 2, MaxMS: 3, MdevMS: 0.5}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(args) != "C\n-n\n-q\n-c\n4\n-W\n1\n--\nexample.com\n" {
		t.Fatalf("args=%q", args)
	}
}

func TestRunFailures(t *testing.T) {
	for _, tc := range []struct{ name, output, diagnostic, status, want string }{
		{"no replies", "4 packets transmitted, 0 received, 100% packet loss, time 3000ms\n", "", "1", "no ICMP replies"},
		{"permissions", "", "ping: socket: Operation not permitted", "2", "ICMP permissions"},
		{"resolver", "", "ping: example.com: Name or service not known", "2", "Name or service not known"},
		{"bad summary", "unsupported output", "", "0", "invalid ping summary"},
		{"missing rtt", "4 packets transmitted, 4 received, 0% packet loss, time 3000ms\n", "", "0", "invalid ping RTT"},
		{"bad counts", "4 packets transmitted, 5 received, 0% packet loss\n", "", "0", "invalid packet counts in ping output"},
		{"wrong count", "3 packets transmitted, 3 received, 0% packet loss\n", "", "0", "invalid packet counts in ping output"},
		{"bad rtt", "4 packets transmitted, 4 received, 0% packet loss\nrtt min/avg/max/mdev = 3.0/2.0/1.0/0.0 ms\n", "", "0", "invalid ping RTT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SUMMARY", tc.output)
			t.Setenv("DIAGNOSTIC", tc.diagnostic)
			t.Setenv("STATUS", tc.status)
			executable(t, `printf '%s' "$SUMMARY"
printf '%s' "$DIAGNOSTIC" >&2
exit "$STATUS"
`)
			got, err := ping.Run(context.Background(), "example.com", 4)
			if err == nil || !strings.Contains(err.Error(), tc.want) || got != (ping.Result{}) {
				t.Fatalf("result=%+v err=%v", got, err)
			}
		})
	}
}

func TestRunMissingTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := ping.Run(context.Background(), "127.0.0.1", 1)
	if err == nil || !strings.Contains(err.Error(), "Install iputils") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunExecutablePermission(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if err := os.WriteFile(filepath.Join(dir, "ping"), []byte("#!/bin/sh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := ping.Run(context.Background(), "127.0.0.1", 1)
	if err == nil || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunContext(t *testing.T) {
	executable(t, "exec /bin/sleep 30\n")
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "timeout", true: "cancel"}[cancelled], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			want := context.DeadlineExceeded
			if cancelled {
				cancel()
				want = context.Canceled
			}
			start := time.Now()
			result, err := ping.Run(ctx, "127.0.0.1", 1)
			if !errors.Is(err, want) || result != (ping.Result{}) || time.Since(start) > time.Second {
				t.Fatalf("result=%+v err=%v duration=%s", result, err, time.Since(start))
			}
		})
	}
}

func TestValidate(t *testing.T) {
	for _, host := range []string{"localhost", "example.com.", "a-b.example", "127.0.0.1", "::1", "fe80::1%eth0"} {
		if err := ping.Validate(host, 1); err != nil {
			t.Errorf("%q: %v", host, err)
		}
	}
	for _, host := range []string{"", "-x", "--help", "a b", "example.com;id", "$(id)", "a\nb", "http://example.com", "a/b", "a..b", "-a.example", "a_.example", strings.Repeat("a", 64) + ".com", "fe80::1%bad zone"} {
		if err := ping.Validate(host, 1); err == nil {
			t.Errorf("accepted %q", host)
		}
	}
	for _, count := range []int{0, -1, 2147483648} {
		if err := ping.Validate("localhost", count); err == nil {
			t.Errorf("accepted count %d", count)
		}
	}
}

func TestRunBoundsToolDiagnostics(t *testing.T) {
	executable(t, `while [ "$i" != 4096 ]; do
 printf '01234567890123456789012345678901' >&2
 i=$((i+1))
 done
 exit 2
 `)
	_, err := ping.Run(context.Background(), "localhost", 1)
	if err == nil || len(err.Error()) > 66000 {
		t.Fatalf("error is absent or exceeds 66000 bytes")
	}
}

func TestRunIPv6WithDuplicateReplies(t *testing.T) {
	t.Setenv("SUMMARY", "1 packets transmitted, 1 received, +1 duplicates, 0% packet loss, time 0ms\nrtt min/avg/max/mdev = 0.010/0.015/0.020/0.005 ms, pipe 2\n")
	executable(t, `printf '%s' "$SUMMARY"`)
	got, err := ping.Run(context.Background(), "::1", 1)
	want := ping.Result{Host: "::1", Transmitted: 1, Received: 1, RTT: ping.RTT{MinMS: 0.010, AvgMS: 0.015, MaxMS: 0.020, MdevMS: 0.005}}
	if err != nil || got != want {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
