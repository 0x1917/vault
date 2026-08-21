package runner

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ruaan-deysel/vault/internal/crypto"
)

func TestDecryptManifestPlaintextPassthrough(t *testing.T) {
	t.Parallel()
	plain := []byte(`{"version":2,"job_name":"legacy"}`)
	got, err := decryptManifest(plain, "")
	if err != nil {
		t.Fatalf("decryptManifest(plaintext) error = %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("plaintext manifest was modified: got %q", got)
	}
}

func TestDecryptManifestEncrypted(t *testing.T) {
	t.Parallel()
	enc, err := crypto.EncryptReader("hunter2", strings.NewReader(`{"job_name":"secret-job"}`))
	if err != nil {
		t.Fatalf("EncryptReader: %v", err)
	}
	cipher, err := io.ReadAll(enc)
	_ = enc.Close()
	if err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}

	got, err := decryptManifest(cipher, "hunter2")
	if err != nil {
		t.Fatalf("decryptManifest error = %v", err)
	}
	if string(got) != `{"job_name":"secret-job"}` {
		t.Errorf("decryptManifest = %q, want plaintext JSON", got)
	}
}

func TestDecryptManifestWrongPassphrase(t *testing.T) {
	t.Parallel()
	enc, _ := crypto.EncryptReader("hunter2", strings.NewReader(`{"job_name":"x"}`))
	cipher, _ := io.ReadAll(enc)
	_ = enc.Close()

	if _, err := decryptManifest(cipher, "wrong"); err == nil {
		t.Error("decryptManifest with wrong passphrase should error")
	}
}

func TestDecryptManifestEncryptedNoPassphrase(t *testing.T) {
	t.Parallel()
	enc, _ := crypto.EncryptReader("hunter2", strings.NewReader(`{"job_name":"x"}`))
	cipher, _ := io.ReadAll(enc)
	_ = enc.Close()

	if _, err := decryptManifest(cipher, ""); err == nil {
		t.Error("decryptManifest with empty passphrase should error")
	}
}
