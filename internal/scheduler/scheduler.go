package scheduler

import (
	"context"
	"log"
	"sort"
	"strconv"
	"time"

	"singbox-node-agent/internal/config"
	"singbox-node-agent/internal/metrics"
	"singbox-node-agent/internal/model"
	"singbox-node-agent/internal/probe"
	"singbox-node-agent/internal/source"
	"singbox-node-agent/internal/util"
)

type Scheduler struct {
	cfg     *config.Config
	src     *source.Manager
	sem     chan struct{}
	nextRun map[string]time.Time
}

func New(cfg *config.Config, src *source.Manager) *Scheduler {
	maxWorkers := cfg.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 10
	}
	return &Scheduler{cfg: cfg, src: src, sem: make(chan struct{}, maxWorkers), nextRun: map[string]time.Time{}}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.TickSeconds) * time.Second)
	defer ticker.Stop()
	s.dispatch(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dispatch(ctx)
		}
	}
}

func (s *Scheduler) dispatch(ctx context.Context) {
	now := time.Now()
	nodes := s.src.ListNodes()
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].NodeID < nodes[j].NodeID })
	active := map[string]struct{}{}
	for _, node := range nodes {
		active[node.Key()] = struct{}{}
		next, ok := s.nextRun[node.Key()]
		if ok && now.Before(next) {
			continue
		}
		s.tryRunOnce(ctx, node)
		interval := node.IntervalSeconds
		if interval <= 0 {
			interval = s.cfg.DefaultProbe.IntervalSeconds
		}
		s.nextRun[node.Key()] = now.Add(time.Duration(interval) * time.Second)
	}
	for key := range s.nextRun {
		if _, ok := active[key]; !ok {
			delete(s.nextRun, key)
		}
	}
}

func (s *Scheduler) tryRunOnce(ctx context.Context, node model.NodeConfig) {
	select {
	case s.sem <- struct{}{}:
		go func() {
			defer func() { <-s.sem }()
			s.runOnce(ctx, node)
		}()
	default:
		log.Printf("[node=%s] skipped: worker pool full", node.NodeID)
	}
}

func (s *Scheduler) runOnce(ctx context.Context, node model.NodeConfig) {
	timeout := node.TimeoutSeconds
	if timeout <= 0 {
		timeout = s.cfg.DefaultProbe.TimeoutSeconds
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	socksPort, err := util.GetFreePort()
	if err != nil {
		log.Printf("[node=%s] get free port failed: %v", node.NodeID, err)
		return
	}

	result := probe.Run(runCtx, node, s.cfg.SingBoxPath, socksPort)
	server, port := node.Server, strconv.Itoa(node.ServerPort)
	metrics.ProbeDuration.WithLabelValues(node.NodeID, server, port, "spawn").Observe(result.SpawnLatency.Seconds())
	metrics.ProbeDuration.WithLabelValues(node.NodeID, server, port, "request").Observe(result.ReqLatency.Seconds())
	if result.HTTPCode > 0 {
		metrics.HTTPStatus.WithLabelValues(node.NodeID, server, port).Set(float64(result.HTTPCode))
	}
	if result.Success {
		metrics.ProbeUp.WithLabelValues(node.NodeID, server, port, result.Phase).Set(1)
		metrics.ProbeTotal.WithLabelValues(node.NodeID, server, port, "success", result.Phase, "").Inc()
		metrics.LastSuccess.WithLabelValues(node.NodeID, server, port).Set(float64(time.Now().Unix()))
		log.Printf("[node=%s][source=%s][name=%s] probe success phase=%s http=%d", node.NodeID, node.Source, node.Name, result.Phase, result.HTTPCode)
		return
	}

	metrics.ProbeUp.WithLabelValues(node.NodeID, server, port, result.Phase).Set(0)
	metrics.ProbeTotal.WithLabelValues(node.NodeID, server, port, "failed", result.Phase, result.ErrorType).Inc()
	log.Printf("[node=%s][source=%s][name=%s] probe failed phase=%s error_type=%s detail=%s err=%v stderr=%q", node.NodeID, node.Source, node.Name, result.Phase, result.ErrorType, result.Detail, result.Err, result.Stderr)
}

func (s *Scheduler) Summary() string {
	return "subscription-aware scheduler with handshake classification started"
}
