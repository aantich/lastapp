package sealer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
)

type credentials struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	AccountHost string `json:"account_host"`
}

// testKey is bytes 0..31; fixedVector was sealed with it by the sealing code
// that preceded this package, pinning the stored envelope format.
var testKey = func() string {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(k)
}()

const fixedVector = `{"v":1,"key_id":"primary-v1","Nonce":"KXxb/29mp/WK4Tj3","Data":"+RK5r/IuFq4dSIKJmHcA6KlOIork9nj3VVGqeissjsybBlpZku/zDqqRGfE1pm/hrYHnybQ3jXl7dIu/dn7gQJzu9UMj3w9o2X+9zRb7033c0Q=="}`

func newSealer(t *testing.T, keyID string) *Sealer {
	t.Helper()
	s, e := New(testKey, keyID)
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestRoundTripBindsOwner(t *testing.T) {
	s := newSealer(t, "")
	value := credentials{Username: "runtime", Password: "secret-value"}
	enc, e := s.Seal("database", "resource-a", "tenant-a", value)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(enc, []byte(value.Password)) {
		t.Fatal("plaintext secret in envelope")
	}
	var out credentials
	if e = s.Unseal("database", "resource-a", "tenant-a", enc, &out); e != nil || out != value {
		t.Fatal("roundtrip", e)
	}
	for _, owner := range [][3]string{{"target", "resource-a", "tenant-a"}, {"database", "resource-b", "tenant-a"}, {"database", "resource-a", "tenant-b"}} {
		if s.Unseal(owner[0], owner[1], owner[2], enc, &out) == nil {
			t.Fatal("envelope accepted a different owner")
		}
	}
	if newSealer(t, "other-key").Unseal("database", "resource-a", "tenant-a", enc, &out) == nil {
		t.Fatal("envelope accepted a different key ID")
	}
}

func TestRejectsTamperedCiphertext(t *testing.T) {
	s := newSealer(t, "")
	enc, e := s.Seal("database", "resource-a", "tenant-a", credentials{Username: "runtime"})
	if e != nil {
		t.Fatal(e)
	}
	var env map[string]any
	if e = json.Unmarshal(enc, &env); e != nil {
		t.Fatal(e)
	}
	data, _ := base64.StdEncoding.DecodeString(env["Data"].(string))
	data[0] ^= 1
	env["Data"] = base64.StdEncoding.EncodeToString(data)
	tampered, _ := json.Marshal(env)
	var out credentials
	if s.Unseal("database", "resource-a", "tenant-a", tampered, &out) == nil {
		t.Fatal("tampering accepted")
	}
}

func TestDecryptsFixedVector(t *testing.T) {
	var out credentials
	if e := newSealer(t, "").Unseal("database", "resource-a", "tenant-a", []byte(fixedVector), &out); e != nil {
		t.Fatal(e)
	}
	if out != (credentials{Username: "runtime", Password: "secret-value"}) {
		t.Fatalf("unexpected plaintext %+v", out)
	}
}

func TestRejectsInvalidKeys(t *testing.T) {
	for _, key := range []string{"", "not base64", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		if _, e := New(key, ""); e == nil {
			t.Fatalf("accepted key %q", key)
		}
	}
}
