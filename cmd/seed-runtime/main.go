// Seed-runtime initializes an isolated standalone environment without an admin UI.
// It must be invoked explicitly after deployment; it is never run during preview.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/lambdawalker/go.apexfission.aws.auth/runtimeconfig"
)

func main() { os.Exit(run(context.Background(), os.Args[1:], os.Stdout, seed)) }
func run(ctx context.Context, args []string, out io.Writer, apply func(context.Context, string, runtimeconfig.Config) error) int {
	fs := flag.NewFlagSet("seed-runtime", flag.ContinueOnError)
	fs.SetOutput(out)
	table := fs.String("table", "", "explicit runtime table name")
	env := fs.String("environment", "", "explicit logical environment")
	defaultsFile := fs.String("defaults-file", "", "runtimeDefaults JSON exported by this standalone deployment")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || strings.TrimSpace(*table) == "" || !runtimeconfig.ValidEnvironment(*env) || *defaultsFile == "" {
		fmt.Fprintln(out, "explicit runtime table, valid environment and deployment defaults file are required")
		return 2
	}
	file, err := os.Open(*defaultsFile)
	if err != nil {
		fmt.Fprintln(out, "cannot read deployment defaults file")
		return 2
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 65536 {
		fmt.Fprintln(out, "invalid defaults file size")
		return 2
	}
	decoder := json.NewDecoder(io.LimitReader(file, 65537))
	decoder.DisallowUnknownFields()
	var defaults runtimeconfig.Config
	if err := decoder.Decode(&defaults); err != nil || defaults.Environment != *env || defaults.Revision != 0 || defaults.Validate() != nil {
		fmt.Fprintln(out, "deployment defaults must be a valid initial configuration for the explicit environment")
		return 2
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		fmt.Fprintln(out, "defaults file must contain one JSON document")
		return 2
	}
	if err := apply(ctx, *table, defaults); err != nil {
		fmt.Fprintln(out, "runtime initialization failed; verify target, access and existing state; no override was replaced")
		return 1
	}
	fmt.Fprintln(out, "runtime initialized or existing configuration preserved")
	return 0
}
func seed(ctx context.Context, table string, defaults runtimeconfig.Config) error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return errors.New("AWS configuration unavailable")
	}
	store := runtimeconfig.Store{DB: dynamodb.NewFromConfig(cfg), Table: table}
	return seedConfiguration(ctx, store, defaults)
}

type configurationStore interface {
	Seed(context.Context, runtimeconfig.Config) error
	Get(context.Context, string) (runtimeconfig.Config, error)
}

func seedConfiguration(ctx context.Context, store configurationStore, defaults runtimeconfig.Config) error {
	err := store.Seed(ctx, defaults)
	if errors.Is(err, runtimeconfig.ErrConflict) {
		existing, e := store.Get(ctx, defaults.Environment)
		if e == nil && existing.Validate() == nil {
			return nil
		}
	}
	return err
}
