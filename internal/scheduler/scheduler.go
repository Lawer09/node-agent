package scheduler

import (
	"context"
	"log"
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
	source  *source.Manager
	sem     chan struct{}
	started bool
}

func New(cfg *config.Config, src *source.Manager) *Scheduler {
	maxWorkers := cfg.Scheduler.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 10
	}
	return &Scheduler{
		cfg:    cfg,
		source: src,
		sem:    make(chan struct{}, maxWorkers),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
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
	nodes := s.source.ListNodes()
	for _, node := range nodes {
		s.tryRunOnce(ctx, node)
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
		log.Printf(
			"[probe][status=skipped] node=%s source=%s name=%q server=%s port=%d reason=%q max_workers=%d",
			node.NodeID,
			node.Source,
			node.Name,
			node.Server,
			node.ServerPort,
			"worker_pool_full",
			cap(s.sem),
		)
	}
}

func (s *Scheduler) runOnce(ctx context.Context, node model.NodeConfig) {

	timeout := node.TimeoutSeconds
	if timeout <= 0 {
		timeout = s.cfg.DefaultProbe.TimeoutSeconds
	}
	if timeout <= 0 {
		timeout = 8
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	socksPort, err := util.GetFreePort()
	if err != nil {
		log.Printf(
			"[probe][status=failed] node=%s source=%s name=%q server=%s port=%d phase=%s error_type=%s classify_reason=%q detail=%q",
			node.NodeID,
			node.Source,
			node.Name,
			node.Server,
			node.ServerPort,
			"spawn",
			"local_port_allocate_failed",
			"failed to allocate local socks port",
			err.Error(),
		)
		return
	}

	targets := probe.ResolveTargets(s.cfg.DefaultProbe)
	result := probe.Run(runCtx, node, s.cfg.SingBoxPath, socksPort, targets)

	server := node.Server
	port := strconv.Itoa(node.ServerPort)

	metrics.NodeInfo.WithLabelValues(
		node.NodeID,
		node.Name,
		node.Source,
		server,
		port,
		node.ServerName,
		node.UTLSFingerprint,
	).Set(1)

	metrics.ProbeDuration.WithLabelValues(node.NodeID, server, port, "spawn").Observe(result.SpawnLatency.Seconds())
	metrics.ProbeDuration.WithLabelValues(node.NodeID, server, port, "request").Observe(result.ReqLatency.Seconds())

	if result.HTTPCode > 0 {
		metrics.HTTPStatus.WithLabelValues(node.NodeID, server, port, result.HTTPClass).Set(float64(result.HTTPCode))
	}

	if result.Success {
		metrics.ProbeUp.WithLabelValues(node.NodeID, server, port, "request").Set(1)
		metrics.ProbeTotal.WithLabelValues(node.NodeID, server, port, "success", "", "").Inc()
		metrics.LastSuccess.WithLabelValues(node.NodeID, server, port).Set(float64(time.Now().Unix()))

		log.Printf(
			"[probe][status=success] node=%s source=%s name=%q server=%s port=%d server_name=%q fp=%q sid=%q pbk_tail=%q timeout=%ds socks_port=%d phase=%s probe_class=%s probe_url=%q http=%d spawn_ms=%d request_ms=%d total_ms=%d",
			node.NodeID,
			node.Source,
			node.Name,
			node.Server,
			node.ServerPort,
			node.ServerName,
			node.UTLSFingerprint,
			maskShortID(node.ShortID),
			tailString(node.PublicKey, 8),
			timeout,
			socksPort,
			result.Phase,
			result.HTTPClass,
			result.TargetURL,
			result.HTTPCode,
			result.SpawnLatency.Milliseconds(),
			result.ReqLatency.Milliseconds(),
			result.TotalLatency.Milliseconds(),
		)
		return
	}

	metrics.ProbeUp.WithLabelValues(node.NodeID, server, port, result.Phase).Set(0)
	metrics.ProbeTotal.WithLabelValues(node.NodeID, server, port, "failed", result.Phase, result.ErrorType).Inc()

	rawErr := ""
	if result.Err != nil {
		rawErr = result.Err.Error()
	}

	log.Printf(
		"[probe][status=failed] node=%s source=%s name=%q server=%s port=%d server_name=%q fp=%q sid=%q pbk_tail=%q timeout=%ds socks_port=%d phase=%s error_type=%s classify_reason=%q probe_class=%s probe_url=%q spawn_ms=%d request_ms=%d total_ms=%d detail=%q raw_error=%q stderr_excerpt=%q",
		node.NodeID,
		node.Source,
		node.Name,
		node.Server,
		node.ServerPort,
		node.ServerName,
		node.UTLSFingerprint,
		maskShortID(node.ShortID),
		tailString(node.PublicKey, 8),
		timeout,
		socksPort,
		result.Phase,
		result.ErrorType,
		result.ClassifyReason,
		result.HTTPClass,
		result.TargetURL,
		result.SpawnLatency.Milliseconds(),
		result.ReqLatency.Milliseconds(),
		result.TotalLatency.Milliseconds(),
		result.Detail,
		rawErr,
		excerpt(result.Stderr, 300),
	)
}

func maskShortID(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return s
	}
	return s[:2] + "***" + s[len(s)-2:]
}

func tailString(s string, n int) string {
	if s == "" {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func excerpt(s string, n int) string {
	if s == "" {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[:n]
}
