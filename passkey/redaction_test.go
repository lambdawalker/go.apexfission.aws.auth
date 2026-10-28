package passkey

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestProviderDetailsNotLogged(t *testing.T) {
	var b bytes.Buffer
	old := log.Writer()
	log.SetOutput(&b)
	defer log.SetOutput(old)
	classify(errors.New("private@example.com token-secret"))
	if strings.Contains(b.String(), "token-secret") || strings.Contains(b.String(), "private@example.com") {
		t.Fatal("provider details exposed")
	}
}
