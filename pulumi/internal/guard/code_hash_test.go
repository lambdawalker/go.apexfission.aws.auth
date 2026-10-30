package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLambdaCodeHashUsesArtifactBytes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lambda.zip")
	os.WriteFile(p, []byte("abc"), 0600)
	hash, e := codeHash(p)
	if e != nil || hash != "ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0=" {
		t.Fatalf("wrong AWS artifact digest %s %v", hash, e)
	}
	os.WriteFile(p, []byte("changed artifact"), 0600)
	changed, e := codeHash(p)
	if e != nil || changed == hash {
		t.Fatal("artifact change ignored")
	}
	if _, e := codeHash(p + "missing"); e == nil {
		t.Fatal("missing artifact accepted")
	}
}
