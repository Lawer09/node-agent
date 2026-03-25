package probe

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	HTTPClass    string
	SpawnLatency time.Duration
	ReqLatency   time.Duration
	Err          error
	Stderr       string
	Detail       string
}

func Run(ctx context.Context, node model.NodeConfig, singBoxPath string, socksPort int, targets []ProbeTarget) Result {
	cfgPath, err := writeTempConfig(node, socksPort)
	if err != nil {
		return Result{
			Success:   false,
			Phase:     "config",
			ErrorType: "config_error",
			Err:       err,
			Detail:    err.Error(),
		}
	}
	defer os.Remove(cfgPath)

	startSpawn := time.Now()

	cmd := exec.CommandContext(ctx, singBoxPath, "run", "-c", cfgPath)
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{
			Success:   false,
			Phase:     "spawn",
			ErrorType: "spawn_failed",
			Err:       err,
			Detail:    err.Error(),
		}
	}

	var stderrBuf bytes.Buffer

	if err := cmd.Start(); err != nil {
		return Result{
			Success:   false,
			Phase:     "spawn",
			ErrorType: "spawn_failed",
			Err:       err,
			Detail:    err.Error(),
		}
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
		return Result{
			Success:      false,
			Phase:        "listen",
			ErrorType:    classifyErr("listen", err, stderrText),
			SpawnLatency: time.Since(startSpawn),
			Err:          err,
			Stderr:       stderrText,
			Detail:       err.Error(),
		}
	}
	spawnLatency := time.Since(startSpawn)

	if len(targets) == 0 {
		targets = []ProbeTarget{
			{Class: "standard", URL: "https://cp.cloudflare.com/generate_204"},
		}
	}

	for _, target := range targets {
		reqStart := time.Now()
		httpCode, err := doHTTPViaSocks(ctx, socksPort, target.URL)
		reqLatency := time.Since(reqStart)

		if err != nil {
			<-doneCopy
			stderrText := stderrBuf.String()
			phase, errType := classifyPhaseAndErr(err, stderrText)
			return Result{
				Success:      false,
				Phase:        phase,
				ErrorType:    errType,
				HTTPCode:     0,
				HTTPClass:    target.Class,
				SpawnLatency: spawnLatency,
				ReqLatency:   reqLatency,
				Err:          err,
				Stderr:       stderrText,
				Detail:       err.Error(),
			}
		}

		if httpCode < 200 || httpCode >= 400 {
			<-doneCopy
			stderrText := stderrBuf.String()
			return Result{
				Success:      false,
				Phase:        "http_response",
				ErrorType:    "http_non_2xx",
				HTTPCode:     httpCode,
				HTTPClass:    target.Class,
				SpawnLatency: spawnLatency,
				ReqLatency:   reqLatency,
				Err:          fmt.Errorf("unexpected status code: %d", httpCode),
				Stderr:       stderrText,
				Detail:       fmt.Sprintf("unexpected status code: %d", httpCode),
			}
		}
	}

	return Result{
		Success:      true,
		Phase:        "http_response",
		ErrorType:    "",
		HTTPCode:     204,
		HTTPClass:    targets[len(targets)-1].Class,
		SpawnLatency: spawnLatency,
		ReqLatency:   0,
		Stderr:       stderrBuf.String(),
	}
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
