package runner

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ruaan-deysel/vault/internal/crypto"
)

// ageHeaderPrefix is the leading bytes of every age-encrypted stream. age
// writes its format header ("age-encryption.org/v1\n") before any ciphertext,
// so this prefix reliably distinguishes an encrypted manifest from a plaintext
// JSON manifest (which begins with '{'). Checking the prefix is cheaper than
// attempting a decrypt and failing on every plaintext manifest during a scan.
const ageHeaderPrefix = "age-encryption.org/"

// decryptManifest decrypts an age-encrypted manifest and returns the plaintext
// JSON. Manifests that are not age-encrypted (legacy plaintext manifests, or
// manifests from unencrypted jobs) are returned unchanged so backward
// compatibility is preserved. A missing or wrong passphrase on an encrypted
// manifest returns an error so the caller can skip it.
func decryptManifest(data []byte, passphrase string) ([]byte, error) {
	if !bytes.HasPrefix(data, []byte(ageHeaderPrefix)) {
		return data, nil
	}
	if passphrase == "" {
		return nil, fmt.Errorf("manifest is encrypted but no passphrase is configured")
	}
	dec, err := crypto.DecryptReader(passphrase, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decrypting manifest: %w", err)
	}
	defer dec.Close()
	return io.ReadAll(dec)
}
