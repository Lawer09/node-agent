package probe

import (
	"encoding/json"
	"fmt"
	"os"

	"singbox-node-agent/internal/model"
)

func writeTempConfig(node model.NodeConfig, socksPort int) (string, error) {
	cfg := map[string]any{
		"log": map[string]any{"level": "warn"},
		"inbounds": []any{map[string]any{
			"type": "socks", "tag": "socks-in", "listen": "127.0.0.1", "listen_port": socksPort,
		}},
		"outbounds": []any{map[string]any{
			"type": "vless", "tag": "probe-out", "server": node.Server, "server_port": node.ServerPort, "uuid": node.UUID,
			"tls": map[string]any{
				"enabled":     true,
				"server_name": node.ServerName,
				"utls":        map[string]any{"enabled": true, "fingerprint": node.UTLSFingerprint},
				"reality":     map[string]any{"enabled": true, "public_key": node.PublicKey, "short_id": node.ShortID},
			},
		}},
		"route": map[string]any{"final": "probe-out"},
	}

	f, err := os.CreateTemp("", "singbox-probe-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp config failed: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return "", fmt.Errorf("encode temp config failed: %w", err)
	}
	return f.Name(), nil
}
