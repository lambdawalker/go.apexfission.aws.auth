// The consumer links modules in Go; the manifest must match before any operation.
package main

import (
	"context"
	"flag"
	"github.com/lambdawalker/go.apexfission.aws.auth/application"
	"github.com/lambdawalker/go.apexfission.aws.auth/auth"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy"
	sales "github.com/lambdawalker/go.apexfission.aws.auth/examples/sales"
	"log"
	"os"
)

func main() {
	op := flag.String("op", "preview", "build, preview, deploy, status")
	env := flag.String("env", "dev", "environment")
	flag.Parse()
	f, e := os.Open("apexfission.json")
	if e != nil {
		log.Fatal(e)
	}
	defer f.Close()
	m, e := deploy.Parse(f)
	if e != nil {
		log.Fatal(e)
	}
	if e = m.ValidateModules(auth.Module{}, sales.Module{}); e != nil {
		log.Fatal(e)
	}
	root, e := os.Getwd()
	if e != nil {
		log.Fatal(e)
	}
	app, e := application.Compose(auth.Module{}, sales.Module{})
	if e != nil {
		log.Fatal(e)
	}
	engine := deploy.Engine{Root: root, Manifest: m}
	if e = engine.Use(app); e != nil {
		log.Fatal(e)
	}
	if e = engine.Execute(context.Background(), *op, *env, deploy.Options{}); e != nil {
		log.Fatal(e)
	}
}
