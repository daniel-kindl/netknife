package commands

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"

	"github.com/daniel-kindl/netknife/internal/cli"
	"github.com/daniel-kindl/netknife/internal/info"
)

func TestInfoOutput(t *testing.T) {
	source := info.Source{Interfaces: func() ([]net.Interface, error) {
		return []net.Interface{{Index: 1, Name: "lo", MTU: 65536, Flags: net.FlagUp | net.FlagLoopback}}, nil
	}, Addresses: func(net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}}, nil
	}}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"info"}, "1: lo\n  MTU: 65536\n  Flags: up, loopback\n  Hardware address: (none)\n  Addresses:\n    127.0.0.1/8\n"},
		{[]string{"info", "--json"}, "{\"command\":\"info\",\"data\":{\"interfaces\":[{\"index\":1,\"name\":\"lo\",\"mtu\":65536,\"flags\":[\"up\",\"loopback\"],\"hardware_address\":\"\",\"addresses\":[\"127.0.0.1/8\"]}]}}\n"},
	} {
		var out, diagnostic bytes.Buffer
		code := cli.Run(context.Background(), tc.args, &out, &diagnostic, "dev", []cli.Command{Info(source)})
		if code != 0 || out.String() != tc.want || diagnostic.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}

func TestInfoValidationAndFailure(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		code   int
		want   string
		called bool
	}{
		{[]string{"info", "extra"}, 2, "netknife: info takes no arguments\n", false},
		{[]string{"info", "--json"}, 1, "netknife: list interfaces: access denied\n", true},
		{[]string{"info", "--unknown"}, 2, "netknife: unknown option \"--unknown\"\n", false},
	} {
		called := false
		source := info.Source{Interfaces: func() ([]net.Interface, error) { called = true; return nil, errors.New("access denied") }}
		var out, diagnostic bytes.Buffer
		code := cli.Run(context.Background(), tc.args, &out, &diagnostic, "dev", []cli.Command{Info(source)})
		if code != tc.code || out.Len() != 0 || diagnostic.String() != tc.want || called != tc.called {
			t.Fatalf("code=%d stdout=%q stderr=%q collected=%v", code, out.String(), diagnostic.String(), called)
		}
	}
}

func TestInfoEmpty(t *testing.T) {
	source := info.Source{Interfaces: func() ([]net.Interface, error) { return nil, nil }}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"info"}, "No network interfaces found.\n"},
		{[]string{"info", "--json"}, "{\"command\":\"info\",\"data\":{\"interfaces\":[]}}\n"},
	} {
		var out, diagnostic bytes.Buffer
		if code := cli.Run(context.Background(), tc.args, &out, &diagnostic, "dev", []cli.Command{Info(source)}); code != 0 || out.String() != tc.want || diagnostic.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
		}
	}
}

func TestInfoDownInterface(t *testing.T) {
	source := info.Source{Interfaces: func() ([]net.Interface, error) { return []net.Interface{{Index: 2, Name: "eth0", MTU: 1500}}, nil }, Addresses: func(net.Interface) ([]net.Addr, error) { return nil, nil }}
	var out, diagnostic bytes.Buffer
	code := cli.Run(context.Background(), []string{"info"}, &out, &diagnostic, "dev", []cli.Command{Info(source)})
	if code != 0 || out.String() != "2: eth0\n  MTU: 1500\n  Flags: (none)\n  Hardware address: (none)\n  Addresses: (none)\n" || diagnostic.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
	}
}

func TestInfoCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := info.Source{Interfaces: func() ([]net.Interface, error) { cancel(); return nil, nil }}
	var out, diagnostic bytes.Buffer
	code := cli.Run(ctx, []string{"info", "--json"}, &out, &diagnostic, "dev", []cli.Command{Info(source)})
	if code != 130 || out.Len() != 0 || diagnostic.String() != "netknife: cancelled\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), diagnostic.String())
	}
}
