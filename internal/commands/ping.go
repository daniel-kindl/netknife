package commands

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/daniel-kindl/netknife/internal/cli"
	"github.com/daniel-kindl/netknife/internal/ping"
)

// Ping creates a command with invocation-local probe options.
func Ping(probe func(context.Context, string, int) (ping.Result, error)) cli.Command {
	return cli.Command{Name: "ping", Arguments: "<host>", Summary: "Check ICMP connectivity to a host", New: func(flags *flag.FlagSet) cli.ExecuteFunc {
		count := flags.Int("count", 4, "Number of ICMP echo probes (1 through 2147483647)")
		return func(ctx context.Context, args []string) (any, error) {
			if len(args) != 1 {
				return nil, &cli.UsageError{Message: "ping requires exactly one host"}
			}
			if err := ping.Validate(args[0], *count); err != nil {
				return nil, &cli.UsageError{Message: err.Error()}
			}
			return probe(ctx, args[0], *count)
		}
	}, Text: func(out io.Writer, data any) error {
		result := data.(ping.Result)
		_, err := fmt.Fprintf(out, "Ping %s: %d transmitted, %d received, %g%% loss\nRTT min/avg/max/mdev: %g/%g/%g/%g ms\n", result.Host, result.Transmitted, result.Received, result.LossPercent, result.RTT.MinMS, result.RTT.AvgMS, result.RTT.MaxMS, result.RTT.MdevMS)
		return err
	}}
}
