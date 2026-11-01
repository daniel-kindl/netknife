package commands

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/daniel-kindl/netknife/internal/cli"
	"github.com/daniel-kindl/netknife/internal/ping"
)

func TestPingOutput(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		want  string
		count int
	}{
		{[]string{"ping", "example.com"}, "Ping example.com: 4 transmitted, 3 received, 25% loss\nRTT min/avg/max/mdev: 1/2/3/0.5 ms\n", 4},
		{[]string{"--json", "ping", "example.com", "--count", "4"}, "{\"command\":\"ping\",\"data\":{\"host\":\"example.com\",\"transmitted\":4,\"received\":3,\"loss_percent\":25,\"rtt\":{\"min_ms\":1,\"avg_ms\":2,\"max_ms\":3,\"mdev_ms\":0.5}}}\n", 4},
		{[]string{"ping", "--count=3", "example.com", "--count", "4"}, "Ping example.com: 4 transmitted, 3 received, 25% loss\nRTT min/avg/max/mdev: 1/2/3/0.5 ms\n", 4},
	} {
		source := func(ctx context.Context, host string, count int) (ping.Result, error) {
			if host != "example.com" || count != tc.count {
				t.Fatalf("host=%q count=%d", host, count)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("missing deadline")
			}
			return ping.Result{Host: host, Transmitted: 4, Received: 3, LossPercent: 25, RTT: ping.RTT{MinMS: 1, AvgMS: 2, MaxMS: 3, MdevMS: 0.5}}, nil
		}
		var out, diagnostic bytes.Buffer
		code := cli.Run(context.Background(), tc.args, &out, &diagnostic, "dev", []cli.Command{Ping(source)})
		if code != 0 || out.String() != tc.want || diagnostic.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}

func TestPingInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"ping"}, {"ping", "a", "b"}, {"ping", "--count", "0", "a"}, {"ping", "--count", "-1", "a"}, {"ping", "--count", "2147483648", "a"}, {"ping", "--count", "bad", "a"},
		{"ping", "--", "-f"}, {"ping", "example.com;id"}, {"ping", "a\nb"}, {"ping", "--unknown", "a"},
	} {
		var out, diagnostic bytes.Buffer
		code := cli.Run(context.Background(), args, &out, &diagnostic, "dev", []cli.Command{Ping(func(context.Context, string, int) (ping.Result, error) {
			t.Fatal("executed invalid arguments")
			return ping.Result{}, nil
		})})
		if code != 2 || out.Len() != 0 || diagnostic.Len() == 0 {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, out.String(), diagnostic.String())
		}
	}
}

func TestPingFailure(t *testing.T) {
	for _, json := range []string{"--json", "--json=false"} {
		var out, diagnostic bytes.Buffer
		code := cli.Run(context.Background(), []string{"ping", "localhost", json}, &out, &diagnostic, "dev", []cli.Command{Ping(func(context.Context, string, int) (ping.Result, error) {
			return ping.Result{}, errors.New("no ICMP replies")
		})})
		if code != 1 || out.Len() != 0 || diagnostic.String() != "netknife: no ICMP replies\n" {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}

func TestPingContext(t *testing.T) {
	for _, cancelled := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		source := func(ctx context.Context, _ string, _ int) (ping.Result, error) {
			if cancelled {
				cancel()
			}
			<-ctx.Done()
			return ping.Result{}, ctx.Err()
		}
		var out, diagnostic bytes.Buffer
		start := time.Now()
		code := cli.Run(ctx, []string{"ping", "localhost", "--timeout", "1ms", "--json"}, &out, &diagnostic, "dev", []cli.Command{Ping(source)})
		cancel()
		wantCode, wantError := 1, "netknife: timeout exceeded\n"
		if cancelled {
			wantCode, wantError = 130, "netknife: cancelled\n"
		}
		if code != wantCode || out.Len() != 0 || diagnostic.String() != wantError || time.Since(start) > time.Second {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}

func TestPingHelp(t *testing.T) {
	for _, args := range [][]string{{"ping", "--help"}, {"help", "ping"}} {
		var out, diagnostic bytes.Buffer
		code := cli.Run(context.Background(), args, &out, &diagnostic, "dev", []cli.Command{Ping(nil)})
		if code != 0 || !strings.Contains(out.String(), "Usage: netknife ping <host> [options]\n") || !strings.Contains(out.String(), "count") || !strings.Contains(out.String(), "default 4") || diagnostic.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}
