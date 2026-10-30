package githubsetup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type deliveryReceipt struct {
	Version                            int
	RequestedVersion, PublishedVersion string
	Secrets                            map[string]string
}
type deliveryEnvelope struct {
	Version     int
	Nonce, Data []byte
}

func (c client) delivery(t Target) (Target, func() error, error) {
	// Without an unlocked vault-derived key, never infer secret equality from public inventory.
	if len(c.o.SecretStateKey) == 0 {
		c.o.RotateSecrets = true
		return t, func() error { return nil }, nil
	}
	if len(c.o.SecretStateKey) != 32 {
		return t, nil, errors.New("invalid secret-delivery metadata key")
	}
	block, e := aes.NewCipher(c.o.SecretStateKey)
	if e != nil {
		return t, nil, e
	}
	aead, e := cipher.NewGCM(block)
	if e != nil {
		return t, nil, e
	}
	aad := []byte("github-delivery-v1\x00" + c.o.Repository + "\x00" + t.Environment)
	name := digest(aad) + ".json"
	path := filepath.Join(c.o.Root, ".attestra", "github-delivery", name)
	next := deliveryReceipt{Version: 1, RequestedVersion: t.SecretVersion, Secrets: map[string]string{}}
	for n, v := range t.Secrets {
		mac := hmac.New(sha256.New, c.o.SecretStateKey)
		mac.Write([]byte(n))
		mac.Write([]byte{0})
		mac.Write([]byte(v))
		next.Secrets[n] = hex.EncodeToString(mac.Sum(nil))
	}
	var prior deliveryReceipt
	if b, err := os.ReadFile(path); err == nil {
		var envelope deliveryEnvelope
		if json.Unmarshal(b, &envelope) != nil || envelope.Version != 1 || len(envelope.Nonce) != aead.NonceSize() {
			return t, nil, errors.New("corrupt encrypted GitHub delivery metadata; inspect/remove this cache to redeliver secrets")
		}
		plain, err := aead.Open(nil, envelope.Nonce, envelope.Data, aad)
		if err != nil { // Vault passphrase rotation changes the derived key; redelivery is safe.
			prior = deliveryReceipt{}
		} else {
			if json.Unmarshal(plain, &prior) != nil {
				clear(plain)
				return t, nil, errors.New("invalid GitHub delivery metadata")
			}
			clear(plain)
		}
	} else if !os.IsNotExist(err) {
		return t, nil, err
	}
	next.PublishedVersion = prior.PublishedVersion
	if prior.Version != 1 || prior.RequestedVersion != next.RequestedVersion || canonical(prior.Secrets) != canonical(next.Secrets) || next.PublishedVersion == "" || c.o.RotateSecrets {
		random := make([]byte, 24)
		if _, e := rand.Read(random); e != nil {
			return t, nil, e
		}
		next.PublishedVersion = hex.EncodeToString(random)
	}
	t.SecretVersion = next.PublishedVersion
	commit := func() error {
		plain, e := json.Marshal(next)
		if e != nil {
			return e
		}
		defer clear(plain)
		nonce := make([]byte, aead.NonceSize())
		if _, e = rand.Read(nonce); e != nil {
			return e
		}
		b, e := json.Marshal(deliveryEnvelope{1, nonce, aead.Seal(nil, nonce, plain, aad)})
		if e != nil {
			return e
		}
		dir := filepath.Dir(path)
		if e = os.MkdirAll(dir, 0700); e != nil {
			return e
		}
		if e = os.Chmod(dir, 0700); e != nil {
			return e
		}
		f, e := os.CreateTemp(dir, ".delivery-*")
		if e != nil {
			return e
		}
		defer os.Remove(f.Name())
		if _, e = f.Write(b); e != nil {
			f.Close()
			return e
		}
		if e = f.Sync(); e != nil {
			f.Close()
			return e
		}
		if e = f.Close(); e != nil {
			return e
		}
		return os.Rename(f.Name(), path)
	}
	return t, commit, nil
}
