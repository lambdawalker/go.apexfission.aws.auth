package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy"
	"os"
	"path/filepath"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: apexfission init|build|preview|deploy|status|teardown [--env dev]")
	}
	op := os.Args[1]
	f := flag.NewFlagSet(op, flag.ContinueOnError)
	manifest := f.String("manifest", "apexfission.json", "application manifest")
	env := f.String("env", "dev", "environment")
	confirm := f.String("confirm", "", "exact application/environment/account")
	destroy := f.Bool("allow-destroy", false, "explicit deletion authorization")
	prod := f.Bool("allow-production", false, "explicit production deletion authorization")
	if e := f.Parse(os.Args[2:]); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if op == "init" {
		file, e := os.OpenFile(*manifest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = file.WriteString(`{"version":1,"application":"my-app","modules":{"auth":"0.1.0"},"builds":[],"environments":{"dev":{"account":"REPLACE_ACCOUNT","region":"us-east-1","backend":"s3://REPLACE_STATE_BUCKET","stack":"dev","directory":"infra"}}}` + "\n")
		closeErr := file.Close()
		if e != nil {
			return e
		}
		return closeErr
	}
	file, e := os.Open(*manifest)
	if e != nil {
		return e
	}
	defer file.Close()
	m, e := deploy.Parse(file)
	if e != nil {
		return e
	}
	root, e := filepath.Abs(filepath.Dir(*manifest))
	if e != nil {
		return e
	}
	return (deploy.Engine{Manifest: m, Root: root}).Execute(context.Background(), op, *env, deploy.Options{Confirm: *confirm, AllowDestroy: *destroy, AllowProduction: *prod})
}
