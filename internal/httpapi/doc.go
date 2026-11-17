// Package httpapi holds the HTTP plumbing shared by every Lastapp surface:
// JSON decode/send, the single error writer, the console-session wrapper,
// pagination shims, request logging, rate limits, trusted-proxy client IPs and
// origin/cookie naming. It sits below internal/control and never imports it.
package httpapi
