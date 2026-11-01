// Command build-lambdas builds the standalone feature's Lambda deployment archives.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/lambdawalker/go.apexfission.aws.auth/artifacts"
)

func run(ctx context.Context, args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("build-lambdas", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "./dist", "directory for Lambda archives")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	return artifacts.Build(ctx, *out)
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
