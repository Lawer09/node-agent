package config

import (
	"fmt"
	"os"

	"singbox-node-agent/internal/model"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr   string             `yaml:"listen_addr"`
	MetricsPath  string             `yaml:"metrics_path"`
	SingBoxPath  string             `yaml:"singbox_path"`
	MaxWorkers   int                `yaml:"max_workers"`
	TickSeconds  int                `yaml:"tick_seconds"`
	DefaultProbe DefaultProbeConfig `yaml:"default_probe"`
	Subscription SubscriptionConfig `yaml:"subscription"`
	Nodes        []model.NodeConfig `yaml:"nodes"`
}

type DefaultProbeConfig struct {
	IntervalSeconds int    `yaml:"interval_seconds"`
	TimeoutSeconds  int    `yaml:"timeout_seconds"`
	ProbeURL        string `yaml:"probe_url"`
	UTLSFingerprint string `yaml:"utls_fingerprint"`
}

type SubscriptionConfig struct {
	URL              string            `yaml:"url"`
	ReloadSeconds    int               `yaml:"reload_seconds"`
	Headers          map[string]string `yaml:"headers"`
	Enabled          bool              `yaml:"enabled"`
	IncludeSchemes   []string          `yaml:"include_schemes"`
	NodeIDPrefix     string            `yaml:"node_id_prefix"`
	OverrideProbeURL string            `yaml:"override_probe_url"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config failed: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":2112"
	}
	if cfg.MetricsPath == "" {
		cfg.MetricsPath = "/metrics"
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 10
	}
	if cfg.TickSeconds <= 0 {
		cfg.TickSeconds = 1
	}
	if cfg.SingBoxPath == "" {
		cfg.SingBoxPath = "/usr/local/bin/sing-box"
	}
	if cfg.DefaultProbe.IntervalSeconds <= 0 {
		cfg.DefaultProbe.IntervalSeconds = 60
	}
	if cfg.DefaultProbe.TimeoutSeconds <= 0 {
		cfg.DefaultProbe.TimeoutSeconds = 8
	}
	if cfg.DefaultProbe.ProbeURL == "" {
		cfg.DefaultProbe.ProbeURL = "https://cp.cloudflare.com/generate_204"
	}
	if cfg.DefaultProbe.UTLSFingerprint == "" {
		cfg.DefaultProbe.UTLSFingerprint = "chrome"
	}
	if cfg.Subscription.ReloadSeconds <= 0 {
		cfg.Subscription.ReloadSeconds = 300
	}
	if len(cfg.Subscription.IncludeSchemes) == 0 {
		cfg.Subscription.IncludeSchemes = []string{"vless"}
	}

	for i := range cfg.Nodes {
		applyDefaults(&cfg.Nodes[i], &cfg)
	}

	return &cfg, nil
}

func applyDefaults(n *model.NodeConfig, cfg *Config) {
	if n.IntervalSeconds <= 0 {
		n.IntervalSeconds = cfg.DefaultProbe.IntervalSeconds
	}
	if n.TimeoutSeconds <= 0 {
		n.TimeoutSeconds = cfg.DefaultProbe.TimeoutSeconds
	}
	if n.ProbeURL == "" {
		n.ProbeURL = cfg.DefaultProbe.ProbeURL
	}
	if n.UTLSFingerprint == "" {
		n.UTLSFingerprint = cfg.DefaultProbe.UTLSFingerprint
	}
}

func ApplyDefaults(n *model.NodeConfig, cfg *Config) {
	applyDefaults(n, cfg)
}
