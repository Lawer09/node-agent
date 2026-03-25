package source

import (
	"context"
	"log"
	"sync"
	"time"

	"singbox-node-agent/internal/config"
	"singbox-node-agent/internal/metrics"
	"singbox-node-agent/internal/model"
)

type Manager struct {
	cfg   *config.Config
	mu    sync.RWMutex
	nodes map[string]model.NodeConfig
}

func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		cfg:   cfg,
		nodes: map[string]model.NodeConfig{},
	}

	for _, node := range cfg.Nodes {
		m.nodes[node.Key()] = node
	}

	metrics.ActiveNodes.Set(float64(len(m.nodes)))
	return m
}

func (m *Manager) Start(ctx context.Context) {
	m.refresh(ctx)

	if !m.cfg.Subscription.Enabled || m.cfg.Subscription.URL == "" {
		<-ctx.Done()
		return
	}

	interval := time.Duration(m.cfg.Subscription.RefreshIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 300 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refresh(ctx)
		}
	}
}

func (m *Manager) ListNodes() []model.NodeConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]model.NodeConfig, 0, len(m.nodes))
	for _, n := range m.nodes {
		out = append(out, n)
	}
	return out
}

func (m *Manager) refresh(ctx context.Context) {
	_ = ctx // 先保留，避免未使用；如果后面订阅函数支持 ctx 再传进去

	merged := make(map[string]model.NodeConfig)

	for _, node := range m.cfg.Nodes {
		merged[node.Key()] = node
	}
	metrics.SourceRefreshTotal.WithLabelValues("static", "success").Inc()

	if m.cfg.Subscription.Enabled && m.cfg.Subscription.URL != "" {
		nodes, err := LoadFromURL(m.cfg.Subscription, m.cfg.DefaultProbe)
		if err != nil {
			metrics.SourceRefreshTotal.WithLabelValues("subscription", "failed").Inc()
			log.Printf("subscription refresh failed: %v", err)
		} else {
			metrics.SourceRefreshTotal.WithLabelValues("subscription", "success").Inc()
			for _, node := range nodes {
				merged[node.Key()] = node
			}
			log.Printf("subscription refresh success: %d nodes", len(nodes))
		}
	}

	m.mu.Lock()
	m.nodes = merged
	m.mu.Unlock()

	metrics.ActiveNodes.Set(float64(len(merged)))
}
