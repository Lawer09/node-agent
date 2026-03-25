package probe

import (
	"context"
	"errors"
	"net"
	"strings"
)

func classifyPhaseAndErr(err error, stderr string) (phase string, errType string, reason string) {
	stderrLower := strings.ToLower(stderr)
	errLower := ""
	if err != nil {
		errLower = strings.ToLower(err.Error())
	}

	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(errLower, "deadline exceeded") {
		if containsAny(stderrLower, "tls", "handshake", "reality") {
			return "tls_clienthello", "tls_handshake_timeout", "context deadline exceeded and stderr matched tls/reality keywords"
		}
		return "proxy_request", "context_deadline_exceeded", "context deadline exceeded during request phase"
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) || containsAny(errLower, "no such host", "lookup ") {
		return "socks_dial", "dns_resolve_failed", "dns resolution failed"
	}

	if containsAny(errLower,
		"connection refused",
		"network is unreachable",
		"no route to host",
		"connectex",
	) {
		return "socks_dial", "tcp_connect_failed", "tcp connect failed"
	}

	if containsAny(errLower, "socks", "proxyconnect", "proxy connection failed") {
		return "socks_dial", "socks_connect_failed", "socks dial/connect failed"
	}

	if containsAny(errLower, "eof") {
		return "proxy_request", "upstream_eof", "error contains eof"
	}

	if containsAny(errLower, "reset by peer", "broken pipe") {
		return "proxy_request", "proxy_io_error", "connection reset/broken pipe during upstream request"
	}

	if containsAny(errLower, "timeout", "i/o timeout") {
		return "proxy_request", "upstream_timeout", "timeout keyword matched"
	}

	if containsAny(stderrLower, "utls", "fingerprint") {
		return "tls_clienthello", "utls_fingerprint_issue", "stderr matched utls/fingerprint keywords"
	}

	if containsAny(stderrLower, "public_key") {
		return "reality_verify", "reality_public_key_mismatch", "stderr matched public_key"
	}
	if containsAny(stderrLower, "short_id") {
		return "reality_verify", "reality_short_id_mismatch", "stderr matched short_id"
	}
	if containsAny(stderrLower, "server_name", "sni") {
		return "reality_verify", "reality_sni_mismatch", "stderr matched server_name/sni"
	}
	if containsAny(stderrLower, "reality") {
		return "reality_verify", "tls_reality_handshake_failed", "stderr matched reality"
	}

	if containsAny(stderrLower, "tls", "handshake", "clienthello", "certificate") ||
		containsAny(errLower, "tls", "handshake", "clienthello", "certificate") {
		return "tls_clienthello", "tls_clienthello_rejected", "tls/clienthello keyword matched"
	}

	return "proxy_request", "unknown_error", "no classifier rule matched"
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
