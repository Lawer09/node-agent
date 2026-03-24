package model

type NodeConfig struct {
	NodeID          string `yaml:"node_id" json:"node_id"`
	Name            string `yaml:"name" json:"name"`
	Source          string `yaml:"source" json:"source"`
	Server          string `yaml:"server" json:"server"`
	ServerPort      int    `yaml:"server_port" json:"server_port"`
	UUID            string `yaml:"uuid" json:"uuid"`
	ServerName      string `yaml:"server_name" json:"server_name"`
	PublicKey       string `yaml:"public_key" json:"public_key"`
	ShortID         string `yaml:"short_id" json:"short_id"`
	UTLSFingerprint string `yaml:"utls_fingerprint" json:"utls_fingerprint"`
	ProbeURL        string `yaml:"probe_url" json:"probe_url"`
	IntervalSeconds int    `yaml:"interval_seconds" json:"interval_seconds"`
	TimeoutSeconds  int    `yaml:"timeout_seconds" json:"timeout_seconds"`
}

func (n NodeConfig) Key() string {
	if n.NodeID != "" {
		return n.NodeID
	}
	return n.Server
}
