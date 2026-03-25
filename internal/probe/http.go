package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

func doHTTPViaSocks(ctx context.Context, socksPort int, target string, engine string, timeoutSeconds int) (int, string, error) {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "curl":
		return doHTTPViaSocksCurl(ctx, socksPort, target, timeoutSeconds)
	case "golang", "go", "":
		return doHTTPViaSocksGo(ctx, socksPort, target)
	default:
		// 未知值时回退到 curl，更贴近你的生产目标
		return doHTTPViaSocksCurl(ctx, socksPort, target, timeoutSeconds)
	}
}

func doHTTPViaSocksGo(ctx context.Context, socksPort int, target string) (int, string, error) {
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	log.Printf("[http-go][start] target=%q socks_addr=%q", target, socksAddr)

	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		log.Printf("[http-go][dialer_error] target=%q socks_addr=%q err=%v", target, socksAddr, err)
		return 0, "", &HTTPProbeError{
			Engine: "golang",
			Stage:  "dialer",
			Detail: fmt.Sprintf("create socks5 dialer failed: %v", err),
		}
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			start := time.Now()
			log.Printf("[http-go][dial_start] target=%q network=%q addr=%q socks_addr=%q", target, network, addr, socksAddr)

			conn, err := dialer.Dial(network, addr)
			if err != nil {
				log.Printf("[http-go][dial_error] target=%q network=%q addr=%q socks_addr=%q cost_ms=%d err=%v",
					target, network, addr, socksAddr, time.Since(start).Milliseconds(), err)
				return nil, err
			}

			log.Printf("[http-go][dial_ok] target=%q network=%q addr=%q socks_addr=%q cost_ms=%d",
				target, network, addr, socksAddr, time.Since(start).Milliseconds())
			return conn, nil
		},
		DisableKeepAlives: true,
		ForceAttemptHTTP2: false,
		TLSNextProto:      map[string]func(string, *tls.Conn) http.RoundTripper{},
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	client := &http.Client{Transport: transport}

	trace := &httptrace.ClientTrace{
		TLSHandshakeStart: func() {
			log.Printf("[http-go][tls_handshake_start] target=%q", target)
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err != nil {
				log.Printf("[http-go][tls_handshake_error] target=%q err=%v", target, err)
				return
			}
			log.Printf("[http-go][tls_handshake_done] target=%q", target)
		},
		GotFirstResponseByte: func() {
			log.Printf("[http-go][first_response_byte] target=%q", target)
		},
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, target, nil)
	if err != nil {
		return 0, "", &HTTPProbeError{
			Engine: "golang",
			Stage:  "request_build",
			Detail: fmt.Sprintf("build request failed: %v", err),
		}
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (node-agent-probe)")
	req.Header.Set("Accept", "*/*")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[http-go][do_error] target=%q cost_ms=%d err=%v", target, time.Since(start).Milliseconds(), err)
		return 0, "", &HTTPProbeError{
			Engine: "golang",
			Stage:  "do",
			Detail: err.Error(),
		}
	}
	defer resp.Body.Close()

	n, copyErr := io.Copy(io.Discard, resp.Body)
	if copyErr != nil {
		log.Printf("[http-go][body_read_error] target=%q status=%d proto=%q read_bytes=%d err=%v",
			target, resp.StatusCode, resp.Proto, n, copyErr)
		return 0, resp.Proto, &HTTPProbeError{
			Engine: "golang",
			Stage:  "body_read",
			Proto:  resp.Proto,
			Detail: copyErr.Error(),
		}
	}

	log.Printf("[http-go][done] target=%q status=%d proto=%q read_bytes=%d total_ms=%d",
		target, resp.StatusCode, resp.Proto, n, time.Since(start).Milliseconds())

	return resp.StatusCode, resp.Proto, nil
}

func doHTTPViaSocksCurl(ctx context.Context, socksPort int, target string, timeoutSeconds int) (int, string, error) {
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	args := []string{
		"--socks5", socksAddr,
		"-I",
		"--max-time", strconv.Itoa(timeoutSeconds),
		"-A", "Mozilla/5.0 (node-agent-probe-curl)",
		"-sS",
		"-o", "/dev/null",
		"-w", "%{http_code} %{http_version}",
		target,
	}

	log.Printf("[http-curl][start] target=%q socks_addr=%q args=%q", target, socksAddr, strings.Join(args, " "))

	start := time.Now()
	cmd := exec.CommandContext(ctx, "curl", args...)
	out, err := cmd.CombinedOutput()
	cost := time.Since(start).Milliseconds()

	raw := strings.TrimSpace(string(out))

	if err != nil {
		exitCode := extractExitCode(err)
		log.Printf("[http-curl][error] target=%q cost_ms=%d exit_code=%d err=%v output=%q", target, cost, exitCode, err, raw)

		stage := "request"
		detail := raw

		if exitCode == 35 {
			stage = "tls_handshake"
			detail = "curl exit 35 tls/ssl connect error: " + raw
		} else if exitCode == 28 {
			stage = "timeout"
			detail = "curl exit 28 operation timeout: " + raw
		} else if exitCode == 7 {
			stage = "tcp_connect"
			detail = "curl exit 7 connect failed: " + raw
		} else if exitCode == 56 {
			stage = "connection_closed"
			detail = "curl exit 56 connection closed/reset: " + raw
		}

		return 0, "", &HTTPProbeError{
			Engine:   "curl",
			Stage:    stage,
			ExitCode: exitCode,
			Detail:   detail,
		}
	}

	parts := strings.Fields(raw)
	if len(parts) < 1 {
		return 0, "", &HTTPProbeError{
			Engine: "curl",
			Stage:  "parse",
			Detail: fmt.Sprintf("unexpected curl output: %q", raw),
		}
	}

	code, convErr := strconv.Atoi(parts[0])
	if convErr != nil {
		return 0, "", &HTTPProbeError{
			Engine: "curl",
			Stage:  "parse",
			Detail: fmt.Sprintf("parse curl http code failed: %v output=%q", convErr, raw),
		}
	}

	proto := ""
	if len(parts) >= 2 {
		proto = parts[1]
	}

	log.Printf("[http-curl][done] target=%q cost_ms=%d status=%d proto=%q", target, cost, code, proto)
	return code, proto, nil
}

func extractExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
