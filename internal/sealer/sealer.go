// Package sealer encrypts registry secrets (database credentials, OIDC flow
// state, data cursors, schema snapshots) into owner-bound envelopes with the
// configured AES-256-GCM key. The envelope format is stored in the registry:
// changing it breaks existing sealed rows.
package sealer

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// DefaultKeyID names the key when the configuration does not.
const DefaultKeyID = "primary-v1"

// Sealer seals and opens envelopes bound to purpose, record and Tenant.
type Sealer struct {
	key   cipher.AEAD
	keyID string
}

// New builds a Sealer from the base64 encoding of a 32-byte key. An empty
// keyID means DefaultKeyID.
func New(encodedKey, keyID string) (*Sealer, error) {
	raw, e := base64.StdEncoding.DecodeString(encodedKey)
	if e != nil {
		// base64 errors report only an input offset, never key material.
		return nil, fmt.Errorf("encryption_key must encode 32 bytes: %w", e)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("encryption_key must encode 32 bytes")
	}
	block, e := aes.NewCipher(raw)
	if e != nil {
		return nil, fmt.Errorf("encryption_key cipher setup failed: %w", e)
	}
	key, e := cipher.NewGCM(block)
	if e != nil {
		return nil, fmt.Errorf("encryption_key cipher setup failed: %w", e)
	}
	if keyID == "" {
		keyID = DefaultKeyID
	}
	return &Sealer{key: key, keyID: keyID}, nil
}

type envelope struct {
	Version     int    `json:"v"`
	KeyID       string `json:"key_id"`
	Nonce, Data []byte
}

// Seal encrypts the JSON encoding of value for its owner.
func (s *Sealer) Seal(purpose, record, tenant string, value any) ([]byte, error) {
	b, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, s.key.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, e
	}
	out := s.key.Seal(nil, nonce, b, []byte(purpose+"|"+record+"|"+tenant))
	return json.Marshal(envelope{1, s.keyID, nonce, out})
}

// Unseal opens an envelope sealed for the same owner and decodes it into out.
func (s *Sealer) Unseal(purpose, record, tenant string, data []byte, out any) error {
	var env envelope
	if e := json.Unmarshal(data, &env); e != nil {
		return fmt.Errorf("invalid encrypted envelope: %w", e)
	}
	if (env.KeyID != "" && env.KeyID != s.keyID) || env.Version != 1 || len(env.Nonce) != s.key.NonceSize() {
		return fmt.Errorf("invalid encrypted envelope")
	}
	b, e := s.key.Open(nil, env.Nonce, env.Data, []byte(purpose+"|"+record+"|"+tenant))
	if e != nil {
		return fmt.Errorf("credential decryption failed: %w", e)
	}
	return json.Unmarshal(b, out)
}
