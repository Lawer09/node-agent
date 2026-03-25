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
	Success        bool
	Phase          string
	ErrorType      string
	HTTPCode       int
	HTTPClass      string
	TargetURL      string
	ClassifyReason string

	SpawnLatency time.Duration
	ReqLatency   time.Duration
	TotalLatency time.Duration

	Err    error
	Stderr string
	Detail string
}

func Run(ctx context.Context, node model.NodeConfig, singBoxPath string, socksPort int, targets []ProbeTarget) Result {
	overallStart := time.Now()

	cfgPath, err := writeTempConfig(node, socksPort)
	if err != nil {
		return Result{
			Success:        false,
			Phase:          "config",
			ErrorType:      "config_error",
			ClassifyReason: "temporary sing-box config generation failed",
			Err:            err,
			Detail:         err.Error(),
			TotalLatency:   time.Since(overallStart),
		}
	}
	defer os.Remove(cfgPath)

	startSpawn := time.Now()

	cmd := exec.CommandContext(ctx, singBoxPath, "run", "-c", cfgPath)
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{
			Success:        false,
			Phase:          "spawn",
			ErrorType:      "spawn_failed",
			ClassifyReason: "failed to create stderr pipe",
			Err:            err,
			Detail:         err.Error(),
			TotalLatency:   time.Since(overallStart),
		}
	}

	var stderrBuf bytes.Buffer

	if err := cmd.Start(); err != nil {
		return Result{
			Success:        false,
			Phase:          "spawn",
			ErrorType:      "spawn_failed",
			ClassifyReason: "failed to start sing-box process",
			Err:            err,
			Detail:         err.Error(),
			TotalLatency:   time.Since(overallStart),
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
			Success:        false,
			Phase:          "listen",
			ErrorType:      "listen_timeout",
			ClassifyReason: "local socks port was not ready before timeout",
			SpawnLatency:   time.Since(startSpawn),
			TotalLatency:   time.Since(overallStart),
			Err:            err,
			Stderr:         stderrText,
			Detail:         err.Error(),
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
			phase, errType, reason := classifyPhaseAndErr(err, stderrText)
			return Result{
				Success:        false,
				Phase:          phase,
				ErrorType:      errType,
				HTTPCode:       0,
				HTTPClass:      target.Class,
				TargetURL:      target.URL,
				ClassifyReason: reason,
				SpawnLatency:   spawnLatency,
				ReqLatency:     reqLatency,
				TotalLatency:   time.Since(overallStart),
				Err:            err,
				Stderr:         stderrText,
				Detail:         err.Error(),
			}
		}

		if httpCode < 200 || httpCode >= 400 {
			<-doneCopy
			stderrText := stderrBuf.String()
			return Result{
				Success:        false,
				Phase:          "http_response",
				ErrorType:      "http_non_2xx",
				HTTPCode:       httpCode,
				HTTPClass:      target.Class,
				TargetURL:      target.URL,
				ClassifyReason: "http response status is not in 2xx/3xx success range expected by probe",
				SpawnLatency:   spawnLatency,
				ReqLatency:     reqLatency,
				TotalLatency:   time.Since(overallStart),
				Err:            fmt.Errorf("unexpected status code: %d", httpCode),
				Stderr:         stderrText,
				Detail:         fmt.Sprintf("unexpected status code: %d", httpCode),
			}
		}
	}

	lastTarget := targets[len(targets)-1]
	return Result{
		Success:        true,
		Phase:          "http_response",
		ErrorType:      "",
		HTTPCode:       204,
		HTTPClass:      lastTarget.Class,
		TargetURL:      lastTarget.URL,
		ClassifyReason: "probe request finished successfully",
		SpawnLatency:   spawnLatency,
		ReqLatency:     0,
		TotalLatency:   time.Since(overallStart),
		Stderr:         stderrBuf.String(),
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

func RunDebugSingbox(ctx context.Context, singBoxPath, cfgPath string, socksPort int, hold time.Duration) error {
	cmd := exec.CommandContext(ctx, singBoxPath, "run", "-c", cfgPath)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe failed: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe failed: %w", err)
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sing-box failed: %w", err)
	}

	go func() {
		_, _ = io.Copy(&stdoutBuf, stdoutPipe)
	}()
	go func() {
		_, _ = io.Copy(&stderrBuf, stderrPipe)
	}()

	if err := waitPortReady(ctx, "127.0.0.1", socksPort, 5*time.Second); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("debug socks not ready: %w, stderr=%q", err, stderrBuf.String())
	}

	log.Printf("[debug] sing-box is ready on 127.0.0.1:%d", socksPort)
	log.Printf("[debug] keeping process alive for %s, press Ctrl+C to stop", hold)

	select {
	case <-ctx.Done():
		log.Printf("[debug] received stop signal")
	case <-time.After(hold):
		log.Printf("[debug] hold time reached")
	}

	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_, _ = cmd.Process.Wait()

	if s := stdoutBuf.String(); s != "" {
		log.Printf("[debug] stdout: %s", s)
	}
	if s := stderrBuf.String(); s != "" {
		log.Printf("[debug] stderr: %s", s)
	}

	return nil
}
