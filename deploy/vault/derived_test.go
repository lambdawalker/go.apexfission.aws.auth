package vault

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestMetadataKeysAreVaultAndDomainScoped(t *testing.T) {
	v, e := Create(filepath.Join(t.TempDir(), "vault"), "repo/prefix", "silver planets wander beyond quiet mountains")
	if e != nil {
		t.Fatal(e)
	}
	a, b := v.DeriveKey("github-delivery"), v.DeriveKey("other")
	if len(a) != 32 || bytes.Equal(a, b) || !bytes.Equal(a, v.DeriveKey("github-delivery")) {
		t.Fatal("invalid domain separation")
	}
	v.Close()
	if v.DeriveKey("github-delivery") != nil {
		t.Fatal("closed vault key reused")
	}
	clear(a)
	clear(b)
}
