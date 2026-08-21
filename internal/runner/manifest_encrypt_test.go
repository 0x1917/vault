package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruaan-deysel/vault/internal/crypto"
	"github.com/ruaan-deysel/vault/internal/db"
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

func TestWriteManifestEncryptsWithPassphrase(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	dest := createLocalDest(t, database, storageDir)
	if err := database.SetSetting("encryption_passphrase", "hunter2"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	job := db.Job{Name: "enc-job", Encryption: "age", Compression: "zstd"}
	basePath := "enc-job/1_2026-01-15_020000"

	r.writeManifest(context.Background(), dest, basePath, job, nil, 1, "full", 1, 0, 500, nil, nil, "2026-01-15_020000", "hunter2")

	raw, err := os.ReadFile(filepath.Join(storageDir, basePath, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte(ageHeaderPrefix)) {
		head := raw
		if len(head) > 64 {
			head = head[:64]
		}
		t.Fatalf("manifest.json is not age-encrypted; head=%q", head)
	}
	if bytes.Contains(raw, []byte("enc-job")) {
		t.Error("encrypted manifest leaks job name")
	}
}

func TestWriteManifestPlaintextWithoutPassphrase(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	dest := createLocalDest(t, database, storageDir)

	job := db.Job{Name: "plain-job", Encryption: "", Compression: "zstd"}
	basePath := "plain-job/1_2026-01-15_020000"

	r.writeManifest(context.Background(), dest, basePath, job, nil, 1, "full", 1, 0, 500, nil, nil, "2026-01-15_020000", "")

	raw, err := os.ReadFile(filepath.Join(storageDir, basePath, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest.json should be plaintext JSON: %v", err)
	}
	if m["job_name"] != "plain-job" {
		t.Errorf("job_name = %v, want plain-job", m["job_name"])
	}
}

func TestScanStorageManifestsDecryptsEncrypted(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	dest := createLocalDest(t, database, storageDir)
	if err := database.SetSetting("encryption_passphrase", "hunter2"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	dir := filepath.Join(storageDir, "enc-job", "1_2026-01-15_020000")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	plain := `{"version":2,"job_name":"enc-job","backup_type":"full","size_bytes":500}`
	enc, err := crypto.EncryptReader("hunter2", strings.NewReader(plain))
	if err != nil {
		t.Fatalf("EncryptReader: %v", err)
	}
	cipher, err := io.ReadAll(enc)
	_ = enc.Close()
	if err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), cipher, 0o644); err != nil {
		t.Fatal(err)
	}

	manifests, err := r.ScanStorageManifests(dest)
	if err != nil {
		t.Fatalf("ScanStorageManifests: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("got %d manifests, want 1", len(manifests))
	}
	if manifests[0]["job_name"] != "enc-job" {
		t.Errorf("job_name = %v, want enc-job", manifests[0]["job_name"])
	}
	if sp, _ := manifests[0]["storage_path"].(string); sp == "" {
		t.Error("storage_path missing")
	}
}

func TestScanStorageManifestsSkipsEncryptedWithoutPassphrase(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	dest := createLocalDest(t, database, storageDir)
	// No encryption_passphrase setting — resolvePassphrase() returns "".

	dir := filepath.Join(storageDir, "enc-job", "1_2026-01-15_020000")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	enc, _ := crypto.EncryptReader("hunter2", strings.NewReader(`{"version":2,"job_name":"enc-job","backup_type":"full"}`))
	cipher, _ := io.ReadAll(enc)
	_ = enc.Close()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), cipher, 0o644); err != nil {
		t.Fatal(err)
	}

	manifests, err := r.ScanStorageManifests(dest)
	if err != nil {
		t.Fatalf("ScanStorageManifests: %v", err)
	}
	if len(manifests) != 0 {
		t.Fatalf("got %d manifests, want 0 (encrypted manifest must be skipped without a passphrase)", len(manifests))
	}
}
