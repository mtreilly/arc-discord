package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type tunnelOptions struct {
	Provider       string
	ListenAddr     string
	NgrokAuthToken string
	NgrokAPI       string
	LocalHost      string
}

type TunnelSession struct {
	Provider string
	URL      string
	stop     func(context.Context) error
}

func (s *TunnelSession) Close(ctx context.Context) error {
	if s == nil || s.stop == nil {
		return nil
	}
	return s.stop(ctx)
}

func startTunnel(ctx context.Context, opts tunnelOptions) (*TunnelSession, error) {
	switch strings.ToLower(strings.TrimSpace(opts.Provider)) {
	case "", "none":
		return nil, nil
	case "ngrok":
		return startNgrokTunnel(ctx, opts)
	case "localtunnel":
		return startLocaltunnel(ctx, opts)
	case "auto":
		session, err := startNgrokTunnel(ctx, opts)
		if err == nil {
			return session, nil
		}
		return startLocaltunnel(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported tunnel provider %q (expected ngrok, localtunnel, auto)", opts.Provider)
	}
}

type process interface {
	Start() error
	Wait() error
	Signal(os.Signal) error
	Kill() error
}

type execProcess struct {
	cmd *exec.Cmd
}

func (p *execProcess) Start() error { return p.cmd.Start() }
func (p *execProcess) Wait() error  { return p.cmd.Wait() }
func (p *execProcess) Signal(sig os.Signal) error {
	if p.cmd.Process == nil {
		return errors.New("process not started")
	}
	return p.cmd.Process.Signal(sig)
}
func (p *execProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

var processFactory = func(ctx context.Context, bin string, args []string, env []string) (process, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return &execProcess{cmd: cmd}, nil
}

func startNgrokTunnel(ctx context.Context, opts tunnelOptions) (*TunnelSession, error) {
	if opts.ListenAddr == "" {
		return nil, errors.New("listen address required for tunnel")
	}
	ngrokAPI := opts.NgrokAPI
	if ngrokAPI == "" {
		ngrokAPI = "http://127.0.0.1:4040/api/tunnels"
	}

	args := []string{"http", opts.ListenAddr, "--log=stdout", "--log-format=json"}
	env := []string{}
	if opts.NgrokAuthToken != "" {
		env = append(env, "NGROK_AUTHTOKEN="+opts.NgrokAuthToken)
	}

	proc, err := processFactory(ctx, "ngrok", args, env)
	if err != nil {
		return nil, fmt.Errorf("launch ngrok: %w", err)
	}
	if err := proc.Start(); err != nil {
		return nil, fmt.Errorf("start ngrok: %w", err)
	}

	url, err := waitForNgrokURL(ctx, ngrokAPI, 15*time.Second)
	if err != nil {
		_ = proc.Kill()
		return nil, fmt.Errorf("ngrok tunnel not ready: %w", err)
	}

	session := &TunnelSession{
		Provider: "ngrok",
		URL:      url,
		stop: func(shutdown context.Context) error {
			if err := proc.Signal(os.Interrupt); err != nil {
				_ = proc.Kill()
				return err
			}
			done := make(chan error, 1)
			go func() { done <- proc.Wait() }()
			select {
			case err := <-done:
				return err
			case <-shutdown.Done():
				return proc.Kill()
			case <-time.After(2 * time.Second):
				return proc.Kill()
			}
		},
	}
	return session, nil
}

var httpClient = &http.Client{Timeout: 2 * time.Second}

func waitForNgrokURL(ctx context.Context, api string, timeout time.Duration) (string, error) {
	if api == "" {
		return "", errors.New("ngrok API endpoint required")
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		url, err := fetchNgrokURL(waitCtx, api)
		if err == nil && url != "" {
			return url, nil
		}
		select {
		case <-waitCtx.Done():
			return "", errors.New("timed out waiting for ngrok public url")
		case <-ticker.C:
			continue
		}
	}
}

type ngrokTunnelResponse struct {
	Tunnels []struct {
		PublicURL string `json:"public_url"`
		Proto     string `json:"proto"`
	} `json:"tunnels"`
}

func fetchNgrokURL(ctx context.Context, api string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ngrok api returned %s", resp.Status)
	}
	var payload ngrokTunnelResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	for _, tunnel := range payload.Tunnels {
		if strings.HasPrefix(tunnel.PublicURL, "https://") {
			return tunnel.PublicURL, nil
		}
	}
	if len(payload.Tunnels) > 0 {
		return payload.Tunnels[0].PublicURL, nil
	}
	return "", errors.New("no tunnels available yet")
}

type ltCommand interface {
	Start() error
	Wait() error
	Signal(os.Signal) error
	Kill() error
	StdoutPipe() (io.ReadCloser, error)
}

type execLTCommand struct {
	cmd *exec.Cmd
}

func (c *execLTCommand) Start() error { return c.cmd.Start() }
func (c *execLTCommand) Wait() error  { return c.cmd.Wait() }
func (c *execLTCommand) Signal(sig os.Signal) error {
	if c.cmd.Process == nil {
		return errors.New("process not started")
	}
	return c.cmd.Process.Signal(sig)
}
func (c *execLTCommand) Kill() error {
	if c.cmd.Process == nil {
		return nil
	}
	return c.cmd.Process.Kill()
}
func (c *execLTCommand) StdoutPipe() (io.ReadCloser, error) {
	return c.cmd.StdoutPipe()
}

var localtunnelFactory = func(ctx context.Context, host string, port string) (ltCommand, error) {
	args := []string{"--port", port, "--print-requests", "false"}
	if host != "" {
		args = append(args, "--local-host", host)
	}
	cmd := exec.CommandContext(ctx, "lt", args...)
	cmd.Stderr = os.Stderr
	return &execLTCommand{cmd: cmd}, nil
}

func startLocaltunnel(ctx context.Context, opts tunnelOptions) (*TunnelSession, error) {
	if opts.ListenAddr == "" {
		return nil, errors.New("listen address required for localtunnel")
	}
	host, port, err := net.SplitHostPort(opts.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen addr %q: %w", opts.ListenAddr, err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	cmd, err := localtunnelFactory(ctx, host, port)
	if err != nil {
		return nil, fmt.Errorf("launch localtunnel: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("localtunnel stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start localtunnel: %w", err)
	}

	url, err := readLocaltunnelURL(ctx, stdout, 15*time.Second)
	if err != nil {
		_ = cmd.Kill()
		return nil, err
	}

	session := &TunnelSession{
		Provider: "localtunnel",
		URL:      url,
		stop: func(shutdown context.Context) error {
			if err := cmd.Signal(os.Interrupt); err != nil {
				_ = cmd.Kill()
				return err
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				return err
			case <-shutdown.Done():
				return cmd.Kill()
			case <-time.After(2 * time.Second):
				return cmd.Kill()
			}
		},
	}
	return session, nil
}

func readLocaltunnelURL(ctx context.Context, r io.Reader, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	scanner := bufio.NewScanner(r)
	for {
		select {
		case <-ctx.Done():
			return "", errors.New("timed out waiting for localtunnel url")
		default:
			if scanner.Scan() {
				line := scanner.Text()
				if idx := strings.Index(line, "https://"); idx >= 0 {
					url := strings.TrimSpace(line[idx:])
					return url, nil
				}
			} else {
				if err := scanner.Err(); err != nil {
					return "", err
				}
				return "", errors.New("localtunnel exited before providing url")
			}
		}
	}
}

var lookPath = exec.LookPath

func resolveTunnelProvider(provider string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "", "none":
		return "", nil
	case "ngrok", "localtunnel":
		return p, nil
	case "auto":
		if hasBinary("ngrok") {
			return "ngrok", nil
		}
		if hasBinary("lt") {
			return "localtunnel", nil
		}
		return "", errors.New("no supported tunnel binary found (install ngrok or localtunnel)")
	default:
		return "", fmt.Errorf("unsupported tunnel provider %q (expected ngrok, localtunnel, auto)", provider)
	}
}

func hasBinary(name string) bool {
	if name == "" {
		return false
	}
	if _, err := lookPath(name); err == nil {
		return true
	}
	return false
}
