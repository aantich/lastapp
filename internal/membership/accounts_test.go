package membership

import (
	"bytes"
	"strings"
	"testing"
)

func TestPasswordWorkIsBounded(t *testing.T) {
	p := NewPasswords(2)
	first, e := p.Work()
	if e != nil {
		t.Fatal(e)
	}
	second, e := p.Work()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Work(); e == nil {
		t.Fatal("password work is not bounded")
	}
	first()
	third, e := p.Work()
	if e != nil {
		t.Fatal("password capacity was not released", e)
	}
	second()
	third()
}

func TestPrepareAccount(t *testing.T) {
	p := NewPasswords(1)
	// Without a password an existing account is selected: normalized email,
	// no verifier, no derivation slot needed.
	release, e := p.Work()
	if e != nil {
		t.Fatal(e)
	}
	existing, e := p.PrepareAccount(AccountInput{Email: " Existing@Example.TEST "})
	if e != nil || existing.Email != "existing@example.test" || existing.Hash != nil || existing.Salt != nil {
		t.Fatal("existing account selection", existing, e)
	}
	// A new account needs the shared derivation bound.
	if _, e = p.PrepareAccount(AccountInput{Name: "New", Email: "new@example.test", Password: "Valid password 8!"}); e == nil || !strings.Contains(e.Error(), "authentication_busy") {
		t.Fatal("account preparation bypassed the derivation bound", e)
	}
	release()
	for _, in := range []AccountInput{
		{Name: "Bad email", Email: "not an email", Password: "Valid password 8!"},
		{Name: " ", Email: "blank@example.test", Password: "Valid password 8!"},
		{Name: strings.Repeat("n", 161), Email: "long@example.test", Password: "Valid password 8!"},
		{Name: "Short", Email: "short@example.test", Password: strings.Repeat("x", MinPasswordLength-1)},
		{Name: "Long", Email: "long@example.test", Password: strings.Repeat("x", MaxPasswordLength+1)},
	} {
		if _, e = p.PrepareAccount(in); e == nil {
			t.Fatalf("accepted %+v", in)
		}
	}
	// Policy errors carry the public length hint.
	if _, e = p.PrepareAccount(AccountInput{Name: "Short", Email: "short@example.test", Password: "short"}); e == nil || !strings.Contains(e.Error(), PasswordLengthHint()) {
		t.Fatal("policy error lacks the public hint", e)
	}
	created, e := p.PrepareAccount(AccountInput{Name: "  New account ", Email: "New@Example.test", Password: "Valid password 8!"})
	if e != nil {
		t.Fatal(e)
	}
	if created.Name != "New account" || created.Email != "new@example.test" || len(created.Salt) != 16 {
		t.Fatal("prepared account", created)
	}
	if !bytes.Equal(created.Hash, PasswordKey("Valid password 8!", created.Salt, PasswordIterations)) {
		t.Fatal("verifier is not derived with the current iterations")
	}
	again, e := p.PrepareAccount(AccountInput{Name: "New account", Email: "new@example.test", Password: "Valid password 8!"})
	if e != nil || bytes.Equal(again.Salt, created.Salt) {
		t.Fatal("salts are not random", e)
	}
}
