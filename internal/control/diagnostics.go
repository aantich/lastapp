package control

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"

	mariadb "github.com/aantich/go-mariadb"
)

// Never log Error(): server messages can include SQL containing credentials.
// Wrapped errors are described by the types along their chain and by
// structured codes (SQL number, network operation, context state) only.
func diagnostic(err error) []any {
	fields := []any{"error_type", fmt.Sprintf("%T", err)}
	if chain := errorTypes(err, nil); len(chain) > 1 {
		fields = append(fields, "error_chain", strings.Join(chain, " > "))
	}
	if errors.Is(err, context.DeadlineExceeded) {
		fields = append(fields, "context", "deadline_exceeded")
	} else if errors.Is(err, context.Canceled) {
		fields = append(fields, "context", "canceled")
	}
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

// errorTypes lists the dynamic types along an error's wrap chain, depth first
// through joined errors, bounded so a pathological chain cannot flood a log line.
func errorTypes(err error, out []string) []string {
	for err != nil && len(out) < 16 {
		out = append(out, fmt.Sprintf("%T", err))
		switch u := err.(type) {
		case interface{ Unwrap() error }:
			err = u.Unwrap()
		case interface{ Unwrap() []error }:
			for _, inner := range u.Unwrap() {
				out = errorTypes(inner, out)
			}
			return out
		default:
			return out
		}
	}
	return out
}
