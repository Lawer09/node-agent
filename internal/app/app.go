package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"singbox-node-agent/internal/config"
	"singbox-node-agent/internal/metrics"
	"singbox-node-agent/internal/scheduler"
	"singbox-node-agent/internal/source"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Run() error {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		if _, err := os.Stat("/opt/singbox-node-agent/config.yaml"); err == nil {
			configPath = "/opt/singbox-node-agent/config.yaml"
		} else {
			configPath = "configs/config.yaml"
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("read config failed: %w", err)
	}

	metrics.MustRegister()

	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srcMgr := source.NewManager(cfg)
	go srcMgr.Start(ctx)

	sch := scheduler.New(cfg, srcMgr)
	go sch.Start(ctx)

	go func() {
		log.Printf("metrics server listening on %s", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("metrics server error: %v", err)
			cancel()
		}
	}()

	<-ctx.Done()

	log.Printf("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown failed: %w", err)
	}

	return nil
}
