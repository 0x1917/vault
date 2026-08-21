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

func TestEncryptManifestEmptyPassphrase(t *testing.T) {
	t.Parallel()
	plain := []byte(`{"job_name":"no-encryption"}`)
	got, err := encryptManifest(plain, "")
	if err != nil {
		t.Fatalf("encryptManifest error = %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("empty passphrase should return plaintext unchanged")
	}
}

func TestEncryptManifestProducesCiphertext(t *testing.T) {
	t.Parallel()
	plain := []byte(`{"job_name":"secret-job","items":[{"name":"db"}]}`)
	got, err := encryptManifest(plain, "hunter2")
	if err != nil {
		t.Fatalf("encryptManifest error = %v", err)
	}
	if !bytes.HasPrefix(got, []byte(ageHeaderPrefix)) {
		head := got
		if len(head) > 64 {
			head = head[:64]
		}
		t.Errorf("ciphertext missing age header prefix; head=%q", head)
	}
	if bytes.Contains(got, []byte("secret-job")) {
		t.Error("ciphertext leaks plaintext job name")
	}
}

func TestEncryptManifestRoundTrip(t *testing.T) {
	t.Parallel()
	plain := []byte(`{"job_name":"round-trip","size_bytes":123}`)
	cipher, err := encryptManifest(plain, "hunter2")
	if err != nil {
		t.Fatalf("encryptManifest: %v", err)
	}
	back, err := decryptManifest(cipher, "hunter2")
	if err != nil {
		t.Fatalf("decryptManifest: %v", err)
	}
	if !bytes.Equal(back, plain) {
		t.Errorf("round-trip mismatch: got %q, want %q", back, plain)
	}
}
