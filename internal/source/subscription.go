package source

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"singbox-node-agent/internal/config"
	"singbox-node-agent/internal/model"
)

func LoadFromURL(cfg config.SubscriptionConfig, defaults config.DefaultProbeConfig) ([]model.NodeConfig, error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.URL) == "" {
		return nil, nil
	}

	resp, err := http.Get(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read subscription failed: %w", err)
	}

	content := strings.TrimSpace(string(body))

	if cfg.EnableBase64Decode {
		if decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content, "\n", "")); err == nil {
			content = string(decoded)
		}
	}

	lines := strings.Split(content, "\n")
	nodes := make([]model.NodeConfig, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "vless://") {
			continue
		}

		node, ok := parseVLESSReality(line, defaults)
		if !ok {
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func parseVLESSReality(raw string, defaults config.DefaultProbeConfig) (model.NodeConfig, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return model.NodeConfig{}, false
	}

	if u.Scheme != "vless" {
		return model.NodeConfig{}, false
	}

	q := u.Query()
	if q.Get("security") != "reality" {
		return model.NodeConfig{}, false
	}

	host := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	if err != nil || port <= 0 {
		return model.NodeConfig{}, false
	}

	serverName := q.Get("servername")
	if serverName == "" {
		serverName = q.Get("sni")
	}
	if serverName == "" {
		return model.NodeConfig{}, false
	}

	fp := q.Get("fp")
	if fp == "" {
		fp = defaults.UTLSFingerprint
	}

	name := strings.TrimPrefix(u.Fragment, "#")
	if name == "" {
		name = host
	}

	nodeID := host + ":" + strconv.Itoa(port)

	return model.NodeConfig{
		NodeID:          nodeID,
		Name:            name,
		Source:          "subscription",
		Server:          host,
		ServerPort:      port,
		UUID:            u.User.Username(),
		ServerName:      serverName,
		PublicKey:       q.Get("pbk"),
		ShortID:         q.Get("sid"),
		UTLSFingerprint: fp,
		IntervalSeconds: defaults.IntervalSeconds,
		TimeoutSeconds:  defaults.TimeoutSeconds,
	}, true
}
