package info

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
)

func TestCollect(t *testing.T) {
	source := Source{
		Interfaces: func() ([]net.Interface, error) {
			return []net.Interface{
				{Index: 2, Name: "eth0", MTU: 1500, Flags: net.FlagUp | net.FlagBroadcast, HardwareAddr: net.HardwareAddr{2, 0, 0, 0, 0, 1}},
				{Index: 1, Name: "lo", MTU: 65536, Flags: net.FlagUp | net.FlagLoopback},
			}, nil
		},
		Addresses: func(iface net.Interface) ([]net.Addr, error) {
			if iface.Name == "lo" {
				return []net.Addr{&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}}, nil
			}
			return []net.Addr{&net.IPNet{IP: net.ParseIP("2001:db8::2"), Mask: net.CIDRMask(64, 128)}, &net.IPNet{IP: net.ParseIP("192.0.2.2"), Mask: net.CIDRMask(24, 32)}}, nil
		},
	}
	got, err := Collect(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	want := Snapshot{Interfaces: []Interface{
		{Index: 1, Name: "lo", MTU: 65536, Flags: []string{"up", "loopback"}, HardwareAddress: "", Addresses: []string{"127.0.0.1/8"}},
		{Index: 2, Name: "eth0", MTU: 1500, Flags: []string{"up", "broadcast"}, HardwareAddress: "02:00:00:00:00:01", Addresses: []string{"192.0.2.2/24", "2001:db8::2/64"}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestEmptyAndDownInterfaces(t *testing.T) {
	for _, tc := range []struct {
		interfaces []net.Interface
		want       Snapshot
	}{
		{nil, Snapshot{Interfaces: []Interface{}}},
		{[]net.Interface{{Index: 3, Name: "eth1", MTU: 1500}}, Snapshot{Interfaces: []Interface{{Index: 3, Name: "eth1", MTU: 1500, Flags: []string{}, Addresses: []string{}}}}},
	} {
		got, err := Collect(context.Background(), Source{Interfaces: func() ([]net.Interface, error) { return tc.interfaces, nil }, Addresses: func(net.Interface) ([]net.Addr, error) { return nil, nil }})
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("got %#v, error %v; want %#v", got, err, tc.want)
		}
	}
}

func TestCollectionFailures(t *testing.T) {
	for _, tc := range []struct {
		source Source
		want   string
	}{
		{Source{Interfaces: func() ([]net.Interface, error) { return nil, errors.New("permission denied") }}, "list interfaces: permission denied"},
		{Source{Interfaces: func() ([]net.Interface, error) { return []net.Interface{{Name: "eth0"}}, nil }, Addresses: func(net.Interface) ([]net.Addr, error) { return nil, errors.New("device disappeared") }}, "read addresses for eth0: device disappeared"},
	} {
		got, err := Collect(context.Background(), tc.source)
		if err == nil || err.Error() != tc.want || len(got.Interfaces) != 0 {
			t.Fatalf("got %#v, error %v", got, err)
		}
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	source := Source{Interfaces: func() ([]net.Interface, error) { cancel(); return []net.Interface{{Name: "lo"}}, nil }, Addresses: func(net.Interface) ([]net.Addr, error) { t.Fatal("read after cancellation"); return nil, nil }}
	if _, err := Collect(ctx, source); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	source.Interfaces = func() ([]net.Interface, error) { t.Fatal("enumerated after cancellation"); return nil, nil }
	if _, err := Collect(ctx, source); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestCancellationAfterAddressRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := Source{Interfaces: func() ([]net.Interface, error) { return []net.Interface{{Name: "lo"}}, nil }, Addresses: func(net.Interface) ([]net.Addr, error) { cancel(); return nil, nil }}
	got, err := Collect(ctx, source)
	if !errors.Is(err, context.Canceled) || len(got.Interfaces) != 0 {
		t.Fatalf("got %#v, error %v", got, err)
	}
}

func TestNoPartialSnapshot(t *testing.T) {
	source := Source{Interfaces: func() ([]net.Interface, error) { return []net.Interface{{Name: "lo"}, {Name: "eth0"}}, nil }, Addresses: func(iface net.Interface) ([]net.Addr, error) {
		if iface.Name == "eth0" {
			return nil, errors.New("device disappeared")
		}
		return nil, nil
	}}
	got, err := Collect(context.Background(), source)
	if err == nil || err.Error() != "read addresses for eth0: device disappeared" || len(got.Interfaces) != 0 {
		t.Fatalf("got %#v, error %v", got, err)
	}
}
