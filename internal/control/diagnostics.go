package control

import (
	"crypto/x509"
	"errors"
	"fmt"
	mariadb "github.com/aantich/go-mariadb"
	"net"
)

// Never log Error(): server messages can include SQL containing credentials.
func diagnostic(err error) []any {
	fields := []any{"error_type", fmt.Sprintf("%T", err)}
	var me *mariadb.MySQLError
	if errors.As(err, &me) {
		fields = append(fields, "sql_number", me.Number, "sql_state", string(me.SQLState[:]))
	}
	var ne *net.OpError
	if errors.As(err, &ne) {
		fields = append(fields, "network_operation", ne.Op, "network", ne.Net, "timeout", ne.Timeout())
	}
	var certificate x509.CertificateInvalidError
	var hostname x509.HostnameError
	var authority x509.UnknownAuthorityError
	if errors.As(err, &certificate) || errors.As(err, &hostname) || errors.As(err, &authority) {
		fields = append(fields, "tls_verification_failed", true)
	}
	return fields
}

func controlAction(action string) bool {
	switch action {
	case "platform.admin", "tenant.read", "tenant.manage", "membership.read", "membership.manage", "authorization.read", "authorization.manage", "database.read", "database.manage", "space.read", "space.manage", "app.read", "app.manage", "microapp.read", "microapp.manage", "audit.read":
		return true
	}
	return false
}
