// Package info collects local interface information without rendering it.
package info

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
)

// Source supplies OS interface data. Its functions run synchronously.
type Source struct {
	Interfaces func() ([]net.Interface, error)
	Addresses  func(net.Interface) ([]net.Addr, error)
}

// SystemSource reads network configuration from the current network namespace.
func SystemSource() Source {
	return Source{Interfaces: net.Interfaces, Addresses: func(iface net.Interface) ([]net.Addr, error) { return iface.Addrs() }}
}

// Snapshot is a point-in-time observation, not an atomic configuration snapshot.
type Snapshot struct {
	Interfaces []Interface `json:"interfaces"`
}

// Interface describes a local interface, including down and loopback devices.
type Interface struct {
	Index           int      `json:"index"`
	Name            string   `json:"name"`
	MTU             int      `json:"mtu"`
	Flags           []string `json:"flags"`
	HardwareAddress string   `json:"hardware_address"`
	Addresses       []string `json:"addresses"`
}

// Collect returns no partial result on failure. Cancellation is checked around
// each OS operation because the standard-library calls do not accept a context.
func Collect(ctx context.Context, source Source) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	interfaces, err := source.Interfaces()
	if contextErr := ctx.Err(); contextErr != nil {
		return Snapshot{}, contextErr
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("list interfaces: %w", err)
	}
	result := Snapshot{Interfaces: make([]Interface, 0, len(interfaces))}
	for _, iface := range interfaces {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		addresses, err := source.Addresses(iface)
		if contextErr := ctx.Err(); contextErr != nil {
			return Snapshot{}, contextErr
		}
		if err != nil {
			return Snapshot{}, fmt.Errorf("read addresses for %s: %w", iface.Name, err)
		}
		values := make([]string, 0, len(addresses))
		for _, address := range addresses {
			values = append(values, address.String())
		}
		sort.Strings(values)
		flags := []string{}
		if iface.Flags != 0 {
			flags = strings.Split(iface.Flags.String(), "|")
		}
		result.Interfaces = append(result.Interfaces, Interface{Index: iface.Index, Name: iface.Name, MTU: iface.MTU, Flags: flags, HardwareAddress: iface.HardwareAddr.String(), Addresses: values})
	}
	sort.Slice(result.Interfaces, func(i, j int) bool {
		left, right := result.Interfaces[i], result.Interfaces[j]
		if left.Index != right.Index {
			return left.Index < right.Index
		}
		return left.Name < right.Name
	})
	return result, nil
}
