package email

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestProviderErrorsNeverLogSecrets(t *testing.T) {
	var b bytes.Buffer
	old := log.Writer()
	log.SetOutput(&b)
	defer log.SetOutput(old)
	operation(context.Background(), "ses_send", errors.New("private@example.com secret-token"))
	if strings.Contains(b.String(), "private@example.com") || strings.Contains(b.String(), "secret-token") {
		t.Fatal("provider detail leaked")
	}
}
