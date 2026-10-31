// Package commands adapts diagnostic results to the CLI.
package commands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/daniel-kindl/netknife/internal/cli"
	"github.com/daniel-kindl/netknife/internal/info"
)

// Info creates a command that reads local interfaces from source.
func Info(source info.Source) cli.Command {
	return cli.Command{Name: "info", Summary: "Show local network interfaces", New: func(*flag.FlagSet) cli.ExecuteFunc {
		return func(ctx context.Context, args []string) (any, error) {
			if len(args) != 0 {
				return nil, &cli.UsageError{Message: "info takes no arguments"}
			}
			return info.Collect(ctx, source)
		}
	}, Text: func(out io.Writer, data any) error { return writeInfo(out, data.(info.Snapshot)) }}
}

func writeInfo(out io.Writer, snapshot info.Snapshot) error {
	var text strings.Builder
	if len(snapshot.Interfaces) == 0 {
		text.WriteString("No network interfaces found.\n")
	}
	for _, iface := range snapshot.Interfaces {
		flags := strings.Join(iface.Flags, ", ")
		if flags == "" {
			flags = "(none)"
		}
		hardware := iface.HardwareAddress
		if hardware == "" {
			hardware = "(none)"
		}
		fmt.Fprintf(&text, "%d: %s\n  MTU: %d\n  Flags: %s\n  Hardware address: %s\n", iface.Index, iface.Name, iface.MTU, flags, hardware)
		if len(iface.Addresses) == 0 {
			text.WriteString("  Addresses: (none)\n")
		} else {
			text.WriteString("  Addresses:\n")
			for _, address := range iface.Addresses {
				fmt.Fprintf(&text, "    %s\n", address)
			}
		}
	}
	_, err := io.WriteString(out, text.String())
	return err
}
