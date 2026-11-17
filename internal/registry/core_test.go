package registry

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"testing"
)

// TestPKCEMatches pins the shared S256 check with fixed vectors (computed
// independently) and against the two inline checks it replaced: agent OAuth
// (sha256.Sum256) and App launch exchange (TokenHash), both compared with
// subtle.ConstantTimeCompare.
func TestPKCEMatches(t *testing.T) {
	vectors := map[string]string{
		"dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWStXmn8":                                 "Anu5CHn_8MRjMS5Rqbp3ZQB7F7kC-rrowmq08ytDkTM",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":                                 "ZtNPunH49FD35FWYhT5Tv8I7vRKQJ8uxMaL0_9eHjNA",
		"lastapp.verifier~with_all-allowed.chars0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ": "TbI0-IUHC2fVjllZbJ45vmSaf7w7gLr3thL8CtKHs_E",
	}
	agentCheck := func(verifier, challenge string) bool {
		sum := sha256.Sum256([]byte(verifier))
		return subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(challenge)) == 1
	}
	launchCheck := func(verifier, challenge string) bool {
		return subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(TokenHash(verifier))), []byte(challenge)) == 1
	}
	for verifier, challenge := range vectors {
		if !PKCEVerifier.MatchString(verifier) || !PKCEChallenge.MatchString(challenge) {
			t.Fatal("vector outside the PKCE formats", verifier)
		}
		for _, c := range []string{challenge, challenge[:42] + "A", challenge + "=", "", vectors["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]} {
			want := c == challenge
			if got := PKCEMatches(verifier, c); got != want || agentCheck(verifier, c) != got || launchCheck(verifier, c) != got {
				t.Fatalf("S256 check differs for %q / %q: shared %v, want %v", verifier, c, got, want)
			}
		}
	}
	if PKCEMatches("", "") || agentCheck("", "") || launchCheck("", "") {
		t.Fatal("empty verifier and challenge matched")
	}
}
