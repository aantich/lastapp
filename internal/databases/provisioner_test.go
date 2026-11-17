package databases

import (
	"errors"
	"fmt"
	"testing"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/registry"
)

// The stored failure keeps a domain error's own code and message, maps known
// server errors to fixed text and never stores a driver message.
func TestProvisionError(t *testing.T) {
	fallback := "connection_or_provisioning_failed"
	for _, c := range []struct {
		err  error
		code string
	}{
		{fmt.Errorf("wrapped: %w", OwnershipConflict()), "ownership_conflict"},
		{registry.Problem(400, "unsupported_version", "MariaDB 10.11 or newer is required."), "unsupported_version"},
		{&mariadb.MySQLError{Number: 1045, Message: "secret detail"}, "authentication_failed"},
		{&mariadb.MySQLError{Number: 1044}, "insufficient_privileges"},
		{&mariadb.MySQLError{Number: 1142}, "insufficient_privileges"},
		{&mariadb.MySQLError{Number: 1227}, "insufficient_privileges"},
		{&mariadb.MySQLError{Number: 1049}, "database_missing"},
		{&mariadb.MySQLError{Number: 1396}, "account_conflict"},
		{&mariadb.MySQLError{Number: 2002, Message: "secret detail"}, fallback},
		{errors.New("dial tcp: secret detail"), fallback},
	} {
		code, message := ProvisionError(c.err)
		if code != c.code || message == "" || (c.code != "ownership_conflict" && c.code != "unsupported_version" && message == c.err.Error()) {
			t.Errorf("%v: got %q %q, want %q", c.err, code, message, c.code)
		}
		var me *mariadb.MySQLError
		if errors.As(c.err, &me) && me.Message != "" && message == me.Message {
			t.Errorf("%v: driver message stored", c.err)
		}
	}
}

func TestGeneratedName(t *testing.T) {
	for name, want := range map[string]bool{
		"la_0123456789abcdef0123456789abcdef":  true,
		"la_0123456789ABCDEF0123456789abcdef":  false,
		"la_0123456789abcdef0123456789abcde":   false,
		"xla_0123456789abcdef0123456789abcdef": false,
		"la_0123456789abcdef0123456789abcdef`": false,
	} {
		if GeneratedName.MatchString(name) != want {
			t.Errorf("%q: want %v", name, want)
		}
	}
}
