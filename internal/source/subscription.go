package source

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"

	"singbox-node-agent/internal/config"
	"singbox-node-agent/internal/model"
)

func FetchSubscription(ctx context.Context, cfg *config.Config) ([]model.NodeConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.Subscription.URL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range cfg.Subscription.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	content := decodeSubscriptionBody(body)
	lines := scanLines(content)
	nodes := make([]model.NodeConfig, 0, len(lines))
	for _, line := range lines {
		node, ok := parseLine(line, cfg)
		if ok {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func decodeSubscriptionBody(body []byte) string {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return ""
	}
	decoders := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
	}
	compact := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, raw)
	for _, decoder := range decoders {
		decoded, err := decoder(compact)
		if err == nil {
			text := strings.TrimSpace(string(decoded))
			if strings.Contains(text, "://") {
				return text
			}
		}
	}
	return raw
}

func scanLines(content string) []string {
	s := bufio.NewScanner(strings.NewReader(content))
	out := []string{}
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func parseLine(line string, cfg *config.Config) (model.NodeConfig, bool) {
	u, err := neturl.Parse(line)
	if err != nil {
		return model.NodeConfig{}, false
	}
	if !allowedScheme(u.Scheme, cfg.Subscription.IncludeSchemes) {
		return model.NodeConfig{}, false
	}
	if u.Scheme != "vless" {
		return model.NodeConfig{}, false
	}

	q := u.Query()
	if strings.ToLower(q.Get("security")) != "reality" {
		return model.NodeConfig{}, false
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil || port <= 0 {
		return model.NodeConfig{}, false
	}

	name, _ := neturl.QueryUnescape(u.Fragment)
	if name == "" {
		name = u.Host
	}

	serverName := firstNonEmpty(q.Get("servername"), q.Get("sni"))
	if serverName == "" {
		return model.NodeConfig{}, false
	}

	node := model.NodeConfig{
		NodeID:          buildNodeID(cfg.Subscription.NodeIDPrefix, name, u.Hostname(), port),
		Name:            name,
		Source:          "subscription",
		Server:          u.Hostname(),
		ServerPort:      port,
		UUID:            u.User.Username(),
		ServerName:      serverName,
		PublicKey:       q.Get("pbk"),
		ShortID:         q.Get("sid"),
		UTLSFingerprint: normalizeFingerprint(q.Get("fp"), cfg.DefaultProbe.UTLSFingerprint),
		ProbeURL:        firstNonEmpty(cfg.Subscription.OverrideProbeURL, cfg.DefaultProbe.ProbeURL),
		IntervalSeconds: cfg.DefaultProbe.IntervalSeconds,
		TimeoutSeconds:  cfg.DefaultProbe.TimeoutSeconds,
	}
	if node.UUID == "" || node.PublicKey == "" || node.ShortID == "" {
		return model.NodeConfig{}, false
	}
	return node, true
}

func allowedScheme(s string, allow []string) bool {
	if len(allow) == 0 {
		return s == "vless"
	}
	for _, x := range allow {
		if strings.EqualFold(s, x) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func normalizeFingerprint(fp, fallback string) string {
	fp = strings.ToLower(strings.TrimSpace(fp))
	switch fp {
	case "chrome", "firefox", "edge", "safari", "360", "qq", "ios", "android", "random", "randomized":
		return fp
	case "":
		if fallback != "" {
			return fallback
		}
		return "chrome"
	default:
		if fallback != "" {
			return fallback
		}
		return "chrome"
	}
}

func buildNodeID(prefix, name, host string, port int) string {
	base := name
	if strings.TrimSpace(base) == "" {
		base = fmt.Sprintf("%s-%d", host, port)
	}
	base = strings.ToLower(base)
	repl := strings.NewReplacer(" ", "-", ">", "-", "%", "-", "/", "-", "_", "-", ".", "-", ":", "-", "#", "-", "|", "-")
	base = repl.Replace(base)
	if prefix != "" {
		return prefix + base
	}
	return base
}
