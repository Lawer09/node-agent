package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"singbox-node-agent/internal/model"
)

type Config struct {
	ListenAddr   string             `yaml:"listen_addr"`
	MetricsPath  string             `yaml:"metrics_path"`
	SingBoxPath  string             `yaml:"singbox_path"`
	Scheduler    SchedulerConfig    `yaml:"scheduler"`
	DefaultProbe DefaultProbeConfig `yaml:"default_probe"`
	Subscription SubscriptionConfig `yaml:"subscription"`
	ProbeAgent   ProbeAgentConfig   `yaml:"probe_agent"`
	Nodes        []model.NodeConfig `yaml:"nodes"`
}

type SchedulerConfig struct {
	MaxWorkers         int `yaml:"max_workers"`
	ReloadEverySeconds int `yaml:"reload_every_seconds"`
	FailThreshold      int `yaml:"fail_threshold"`
	RecoverThreshold   int `yaml:"recover_threshold"`
}

type DefaultProbeConfig struct {
	IntervalSeconds int          `yaml:"interval_seconds"`
	TimeoutSeconds  int          `yaml:"timeout_seconds"`
	ProbeMode       string       `yaml:"probe_mode"`
	ProbeTargets    ProbeTargets `yaml:"probe_targets"`
	UTLSFingerprint string       `yaml:"utls_fingerprint"`
}

type ProbeTargets struct {
	Standard []string `yaml:"standard"`
	Business []string `yaml:"business"`
}

type SubscriptionConfig struct {
	Enabled                bool     `yaml:"enabled"`
	URL                    string   `yaml:"url"`
	RefreshIntervalSeconds int      `yaml:"refresh_interval_seconds"`
	EnableBase64Decode     bool     `yaml:"enable_base64_decode"`
	IncludeProtocols       []string `yaml:"include_protocols"`
	RealityOnly            bool     `yaml:"reality_only"`
}

type ProbeAgentConfig struct {
	Name     string `yaml:"name"`
	Region   string `yaml:"region"`
	Country  string `yaml:"country"`
	City     string `yaml:"city"`
	Provider string `yaml:"provider"`
	ASN      string `yaml:"asn"`
	Env      string `yaml:"env"`
	Cluster  string `yaml:"cluster"`
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

	applyGlobalDefaults(&cfg)

	for i := range cfg.Nodes {
		applyNodeDefaults(&cfg.Nodes[i], &cfg)
	}

	return &cfg, nil
}

func applyGlobalDefaults(cfg *Config) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":2112"
	}
	if cfg.MetricsPath == "" {
		cfg.MetricsPath = "/metrics"
	}
	if cfg.SingBoxPath == "" {
		cfg.SingBoxPath = "/usr/local/bin/sing-box"
	}

	if cfg.Scheduler.MaxWorkers <= 0 {
		cfg.Scheduler.MaxWorkers = 10
	}
	if cfg.Scheduler.ReloadEverySeconds <= 0 {
		cfg.Scheduler.ReloadEverySeconds = 60
	}
	if cfg.Scheduler.FailThreshold <= 0 {
		cfg.Scheduler.FailThreshold = 3
	}
	if cfg.Scheduler.RecoverThreshold <= 0 {
		cfg.Scheduler.RecoverThreshold = 2
	}

	if cfg.DefaultProbe.IntervalSeconds <= 0 {
		cfg.DefaultProbe.IntervalSeconds = 60
	}
	if cfg.DefaultProbe.TimeoutSeconds <= 0 {
		cfg.DefaultProbe.TimeoutSeconds = 8
	}
	if cfg.DefaultProbe.ProbeMode == "" {
		cfg.DefaultProbe.ProbeMode = "standard"
	}
	if cfg.DefaultProbe.UTLSFingerprint == "" {
		cfg.DefaultProbe.UTLSFingerprint = "chrome"
	}
	if len(cfg.DefaultProbe.ProbeTargets.Standard) == 0 {
		cfg.DefaultProbe.ProbeTargets.Standard = []string{
			"https://cp.cloudflare.com/generate_204",
			"https://www.gstatic.com/generate_204",
		}
	}
	if len(cfg.DefaultProbe.ProbeTargets.Business) == 0 {
		cfg.DefaultProbe.ProbeTargets.Business = []string{}
	}

	if cfg.Subscription.RefreshIntervalSeconds <= 0 {
		cfg.Subscription.RefreshIntervalSeconds = 300
	}
}

func applyNodeDefaults(n *model.NodeConfig, cfg *Config) {
	if n.IntervalSeconds <= 0 {
		n.IntervalSeconds = cfg.DefaultProbe.IntervalSeconds
	}
	if n.TimeoutSeconds <= 0 {
		n.TimeoutSeconds = cfg.DefaultProbe.TimeoutSeconds
	}
	if n.UTLSFingerprint == "" {
		n.UTLSFingerprint = cfg.DefaultProbe.UTLSFingerprint
	}
}
