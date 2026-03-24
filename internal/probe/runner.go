package probe

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"singbox-node-agent/internal/model"
)

type Result struct {
	Success      bool
	Phase        string
	ErrorType    string
	HTTPCode     int
	SpawnLatency time.Duration
	ReqLatency   time.Duration
	Err          error
	Stderr       string
	Detail       string
}

func Run(ctx context.Context, node model.NodeConfig, singBoxPath string, socksPort int) Result {
	cfgPath, err := writeTempConfig(node, socksPort)
	if err != nil {
		return Result{Success: false, Phase: "config", ErrorType: "config_error", Err: err, Detail: err.Error()}
	}
	defer os.Remove(cfgPath)

	startSpawn := time.Now()
	cmd := exec.CommandContext(ctx, singBoxPath, "run", "-c", cfgPath)
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{Success: false, Phase: "spawn", ErrorType: "spawn_failed", Err: err, Detail: err.Error()}
	}

	var stderrBuf bytes.Buffer
	if err := cmd.Start(); err != nil {
		return Result{Success: false, Phase: "spawn", ErrorType: "spawn_failed", Err: err, Detail: err.Error()}
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_, _ = cmd.Process.Wait()
	}()

	doneCopy := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stderrBuf, stderrPipe)
		close(doneCopy)
	}()

	if err := waitPortReady(ctx, "127.0.0.1", socksPort, 3*time.Second); err != nil {
		<-doneCopy
		stderrText := stderrBuf.String()
		return Result{Success: false, Phase: "listen", ErrorType: classifyListenErr(err), SpawnLatency: time.Since(startSpawn), Err: err, Stderr: stderrText, Detail: err.Error()}
	}
	spawnLatency := time.Since(startSpawn)

	reqStart := time.Now()
	httpCode, err := doHTTPViaSocks(ctx, socksPort, node.ProbeURL)
	reqLatency := time.Since(reqStart)
	if err != nil {
		<-doneCopy
		stderrText := stderrBuf.String()
		phase, errType := classifyPhaseAndErr(err, stderrText)
		return Result{Success: false, Phase: phase, ErrorType: errType, HTTPCode: 0, SpawnLatency: spawnLatency, ReqLatency: reqLatency, Err: err, Stderr: stderrText, Detail: err.Error()}
	}
	if httpCode < 200 || httpCode >= 400 {
		<-doneCopy
		stderrText := stderrBuf.String()
		return Result{Success: false, Phase: "http_response", ErrorType: "http_non_2xx", HTTPCode: httpCode, SpawnLatency: spawnLatency, ReqLatency: reqLatency, Err: fmt.Errorf("unexpected status code: %d", httpCode), Stderr: stderrText, Detail: fmt.Sprintf("unexpected status code: %d", httpCode)}
	}

	log.Printf("[node=%s] sing-box stderr: %s", node.NodeID, stderrBuf.String())
	return Result{Success: true, Phase: "http_response", HTTPCode: httpCode, SpawnLatency: spawnLatency, ReqLatency: reqLatency, Stderr: stderrBuf.String()}
}

func waitPortReady(ctx context.Context, host string, port int, timeout time.Duration) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = c.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("port not ready: %s", addr)
}
