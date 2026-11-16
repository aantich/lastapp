package control

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestCredentialEnvelopeBindsOwner(t *testing.T) {
	block, _ := aes.NewCipher(make([]byte, 32))
	key, _ := cipher.NewGCM(block)
	s := &Server{key: key}
	value := credentials{Username: "runtime", Password: "secret-value"}
	enc, e := s.seal("database", "resource-a", "tenant-a", value)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(enc, []byte(value.Password)) {
		t.Fatal("plaintext secret in envelope")
	}
	var out credentials
	if e = s.unseal("database", "resource-a", "tenant-a", enc, &out); e != nil || out != value {
		t.Fatal("roundtrip", e)
	}
	for _, owner := range [][3]string{{"target", "resource-a", "tenant-a"}, {"database", "resource-b", "tenant-a"}, {"database", "resource-a", "tenant-b"}} {
		if e = s.unseal(owner[0], owner[1], owner[2], enc, &out); e == nil {
			t.Fatal("envelope accepted a different owner")
		}
	}
	var env map[string]any
	if e := json.Unmarshal(enc, &env); e != nil {
		t.Fatal(e)
	}
	env["Data"] = "AAAA"
	tampered, _ := json.Marshal(env)
	if s.unseal("database", "resource-a", "tenant-a", tampered, &out) == nil {
		t.Fatal("tampering accepted")
	}
}

func TestRequestValidation(t *testing.T) {
	for _, body := range []string{`{"unknown":1}`, `{} {}`, `{`} {
		r := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
		if decode(httptest.NewRecorder(), r, &struct {
			Name string `json:"name"`
		}{}) == nil {
			t.Fatalf("accepted invalid payload %s", body)
		}
	}
	for _, v := range []string{"", "bad", "00000000-0000-0000-0000-000000000000", "10000000000040008000000000000001"} {
		if id(v) == nil {
			t.Fatal("accepted noncanonical ID", v)
		}
	}
}
