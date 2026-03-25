package probe

import (
	"context"
	"errors"
	"net"
	"strings"
)

func classifyPhaseAndErr(err error, stderr string) (string, string) {
	stderrLower := strings.ToLower(stderr)
	errLower := ""
	if err != nil {
		errLower = strings.ToLower(err.Error())
	}

	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(errLower, "deadline exceeded") {
		if containsAny(stderrLower, "tls", "handshake", "reality") {
			return "tls_clienthello", "tls_handshake_timeout"
		}
		return "proxy_request", "context_deadline_exceeded"
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) || containsAny(errLower, "no such host", "lookup ") {
		return "socks_dial", "dns_resolve_failed"
	}

	if containsAny(errLower,
		"connection refused",
		"network is unreachable",
		"no route to host",
		"connectex",
	) {
		return "socks_dial", "tcp_connect_failed"
	}

	if containsAny(errLower, "socks", "proxyconnect", "proxy connection failed") {
		return "socks_dial", "socks_connect_failed"
	}

	if containsAny(stderrLower, "utls", "fingerprint") {
		return "tls_clienthello", "utls_fingerprint_issue"
	}

	if containsAny(stderrLower, "public_key") {
		return "reality_verify", "reality_public_key_mismatch"
	}
	if containsAny(stderrLower, "short_id") {
		return "reality_verify", "reality_short_id_mismatch"
	}
	if containsAny(stderrLower, "server_name", "sni") {
		return "reality_verify", "reality_sni_mismatch"
	}
	if containsAny(stderrLower, "reality") {
		return "reality_verify", "tls_reality_handshake_failed"
	}

	if containsAny(stderrLower, "tls", "handshake", "clienthello", "certificate") ||
		containsAny(errLower, "tls", "handshake", "clienthello", "certificate") {
		return "tls_clienthello", "tls_clienthello_rejected"
	}

	if containsAny(errLower, "timeout", "i/o timeout", "eof", "reset by peer", "broken pipe") {
		return "proxy_request", "upstream_timeout"
	}

	return "proxy_request", "unknown_error"
}

func classifyErr(stage string, err error, stderr string) string {
	switch stage {
	case "listen":
		return "listen_timeout"
	default:
		_, errType := classifyPhaseAndErr(err, stderr)
		return errType
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
