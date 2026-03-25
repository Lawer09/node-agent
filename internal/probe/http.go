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
	"time"

	"golang.org/x/net/proxy"
)

func doHTTPViaSocks(ctx context.Context, socksPort int, target string) (int, error) {
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	log.Printf("[http][start] target=%q socks_addr=%q", target, socksAddr)

	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		log.Printf("[http][dialer_error] target=%q socks_addr=%q err=%v", target, socksAddr, err)
		return 0, fmt.Errorf("create socks5 dialer failed: %w", err)
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			start := time.Now()
			log.Printf("[http][dial_start] target=%q network=%q addr=%q socks_addr=%q", target, network, addr, socksAddr)

			conn, err := dialer.Dial(network, addr)
			if err != nil {
				log.Printf("[http][dial_error] target=%q network=%q addr=%q socks_addr=%q cost_ms=%d err=%v",
					target, network, addr, socksAddr, time.Since(start).Milliseconds(), err)
				return nil, err
			}

			log.Printf("[http][dial_ok] target=%q network=%q addr=%q socks_addr=%q cost_ms=%d",
				target, network, addr, socksAddr, time.Since(start).Milliseconds())
			return conn, nil
		},
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
	}

	trace := &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			log.Printf("[httptrace][get_conn] target=%q host_port=%q", target, hostPort)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			log.Printf("[httptrace][got_conn] target=%q reused=%v was_idle=%v idle_time=%s",
				target, info.Reused, info.WasIdle, info.IdleTime)
		},
		TLSHandshakeStart: func() {
			log.Printf("[httptrace][tls_handshake_start] target=%q", target)
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err != nil {
				log.Printf("[httptrace][tls_handshake_error] target=%q err=%v", target, err)
				return
			}
			log.Printf("[httptrace][tls_handshake_done] target=%q", target)
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err != nil {
				log.Printf("[httptrace][wrote_request_error] target=%q err=%v", target, info.Err)
				return
			}
			log.Printf("[httptrace][wrote_request_ok] target=%q", target)
		},
		GotFirstResponseByte: func() {
			log.Printf("[httptrace][first_response_byte] target=%q", target)
		},
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, target, nil)
	if err != nil {
		log.Printf("[http][request_build_error] target=%q err=%v", target, err)
		return 0, fmt.Errorf("build request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (node-agent-probe)")
	req.Header.Set("Accept", "*/*")

	start := time.Now()
	log.Printf("[http][do_start] target=%q method=%s", target, req.Method)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[http][do_error] target=%q cost_ms=%d err=%v", target, time.Since(start).Milliseconds(), err)
		return 0, fmt.Errorf("http do failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[http][headers_ok] target=%q cost_ms=%d status=%d proto=%q content_length=%d",
		target,
		time.Since(start).Milliseconds(),
		resp.StatusCode,
		resp.Proto,
		resp.ContentLength,
	)

	n, copyErr := io.Copy(io.Discard, resp.Body)
	if copyErr != nil {
		log.Printf("[http][body_read_error] target=%q status=%d proto=%q read_bytes=%d err=%v",
			target, resp.StatusCode, resp.Proto, n, copyErr)
		return 0, fmt.Errorf("read response body failed: %w", copyErr)
	}

	log.Printf("[http][done] target=%q status=%d proto=%q read_bytes=%d total_ms=%d",
		target,
		resp.StatusCode,
		resp.Proto,
		n,
		time.Since(start).Milliseconds(),
	)

	return resp.StatusCode, nil
}
