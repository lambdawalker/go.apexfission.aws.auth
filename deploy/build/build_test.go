package build

import (
	"archive/zip"
	"context"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveHasExecutableBootstrap(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "binary")
	os.WriteFile(binary, []byte("ELF-test"), 0600)
	archive := filepath.Join(dir, "lambda.zip")
	if e := PackageLambda(binary, archive); e != nil {
		t.Fatal(e)
	}
	z, e := zip.OpenReader(archive)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	if len(z.File) != 1 || z.File[0].Name != "bootstrap" || z.File[0].Mode().Perm() != 0755 {
		t.Fatal(z.File)
	}
}
func TestBuildRejectsEscapeAndDuplicatesBeforeExecution(t *testing.T) {
	for _, targets := range [][]core.BuildTarget{{{Name: "x", Package: "../escape"}}, {{Name: "../x", Package: "./cmd"}}, {{Name: "x", Package: "./cmd"}, {Name: "x", Package: "./cmd"}}} {
		root := t.TempDir()
		os.Mkdir(filepath.Join(root, "cmd"), 0755)
		if e := Lambdas(context.Background(), root, targets); e == nil {
			t.Fatal("invalid build accepted")
		}
	}
}
