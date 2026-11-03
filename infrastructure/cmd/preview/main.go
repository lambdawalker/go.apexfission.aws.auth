package main

import (
	"fmt"
	infra "github.com/lambdawalker/go.apexfission.aws.auth/infrastructure"
	"github.com/lambdawalker/go.infra.composer/adapters/pulumi/reportpreview"
	"os"
)

func main() {
	if err := reportpreview.RunCLI(os.Args[1:], "authentication", "go.apexfission.aws.auth", infra.Preview, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
