package debugmode

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"singbox-node-agent/internal/config"
	"singbox-node-agent/internal/model"
	"singbox-node-agent/internal/probe"
	"singbox-node-agent/internal/source"
)

type Options struct {
	NodeID      string
	Server      string
	Port        int
	SocksPort   int
	HoldSeconds int
	ConfigPath  string
}

func Run(opts Options) error {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}
	if configPath == "" {
		configPath = "/opt/singbox-node-agent/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("read config failed: %w", err)
	}

	nodes := make([]model.NodeConfig, 0, len(cfg.Nodes))
	nodes = append(nodes, cfg.Nodes...)

	if cfg.Subscription.Enabled && cfg.Subscription.URL != "" {
		subNodes, err := source.LoadFromURL(cfg.Subscription, cfg.DefaultProbe)
		if err != nil {
			log.Printf("[debug] load subscription failed: %v", err)
		} else {
			nodes = append(nodes, subNodes...)
		}
	}

	node, err := findNode(nodes, opts)
	if err != nil {
		return err
	}

	log.Printf("[debug] selected node=%s source=%s name=%q server=%s port=%d server_name=%q fp=%q",
		node.NodeID, node.Source, node.Name, node.Server, node.ServerPort, node.ServerName, node.UTLSFingerprint,
	)

	debugCfgPath, err := config.WriteDebugConfig(node, opts.SocksPort)
	if err != nil {
		return fmt.Errorf("write debug config failed: %w", err)
	}

	log.Printf("[debug] sing-box config written to: %s", debugCfgPath)
	log.Printf("[debug] local socks5: 127.0.0.1:%d", opts.SocksPort)
	log.Printf("[debug] test with:")
	log.Printf(`[debug] curl --socks5 127.0.0.1:%d -I --max-time 8 https://www.google.com`, opts.SocksPort)
	log.Printf(`[debug] curl --socks5 127.0.0.1:%d -I --max-time 8 https://cp.cloudflare.com/generate_204`, opts.SocksPort)

	hold := time.Duration(opts.HoldSeconds) * time.Second
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return probe.RunDebugSingbox(ctx, cfg.SingBoxPath, debugCfgPath, opts.SocksPort, hold)
}

func findNode(nodes []model.NodeConfig, opts Options) (model.NodeConfig, error) {
	if opts.NodeID != "" {
		for _, n := range nodes {
			if n.NodeID == opts.NodeID {
				return n, nil
			}
		}
		return model.NodeConfig{}, fmt.Errorf("node not found by node-id: %s", opts.NodeID)
	}

	if opts.Server != "" && opts.Port > 0 {
		for _, n := range nodes {
			if n.Server == opts.Server && n.ServerPort == opts.Port {
				return n, nil
			}
		}
		return model.NodeConfig{}, fmt.Errorf("node not found by server/port: %s:%d", opts.Server, opts.Port)
	}

	return model.NodeConfig{}, fmt.Errorf("please provide --node-id or --server with --port")
}
