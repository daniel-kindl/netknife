package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/daniel-kindl/netknife/internal/cli"
	"github.com/daniel-kindl/netknife/internal/commands"
	"github.com/daniel-kindl/netknife/internal/info"
	"github.com/daniel-kindl/netknife/internal/ping"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, version, []cli.Command{commands.Info(info.SystemSource()), commands.Ping(ping.Run)})
}
