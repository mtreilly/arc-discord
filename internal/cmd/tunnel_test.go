package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProcess struct {
	started int32
	signals int32
	kills   int32
}

func (f *fakeProcess) Start() error {
	atomic.AddInt32(&f.started, 1)
	return nil
}

func (f *fakeProcess) Wait() error { return nil }

func (f *fakeProcess) Signal(_ os.Signal) error {
	atomic.AddInt32(&f.signals, 1)
	return nil
}

func (f *fakeProcess) Kill() error {
	atomic.AddInt32(&f.kills, 1)
	return nil
}

func TestStartNgrokTunnel(t *testing.T) {
	fake := &fakeProcess{}
	originalFactory := processFactory
	processFactory = func(ctx context.Context, bin string, args []string, env []string) (process, error) {
		return fake, nil
	}
	defer func() { processFactory = originalFactory }()

	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tunnels":[{"public_url":"https://cli.ngrok.dev"}]}`))
	}))
	defer server.Close()
	httpClient = server.Client()

	ctx := context.Background()
	session, err := startNgrokTunnel(ctx, tunnelOptions{
		Provider:   "ngrok",
		ListenAddr: "127.0.0.1:8080",
		NgrokAPI:   server.URL,
	})
	if err != nil {
		t.Fatalf("startNgrokTunnel: %v", err)
	}
	if session == nil || session.URL != "https://cli.ngrok.dev" {
		t.Fatalf("unexpected tunnel session: %#v", session)
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := session.Close(closeCtx); err != nil {
		t.Fatalf("close tunnel: %v", err)
	}
	if atomic.LoadInt32(&fake.signals) == 0 {
		t.Fatalf("expected signal to be sent to process")
	}
	if atomic.LoadInt32(&requestCount) == 0 {
		t.Fatalf("expected ngrok API to be queried")
	}
}

func TestStartTunnelUnsupportedProvider(t *testing.T) {
	_, err := startTunnel(context.Background(), tunnelOptions{
		Provider:   "unknown",
		ListenAddr: "127.0.0.1:8080",
	})
	if err == nil {
		t.Fatalf("expected error for unsupported provider")
	}
}

type stubLTCommand struct {
	output string
}

func (s *stubLTCommand) Start() error           { return nil }
func (s *stubLTCommand) Wait() error            { return nil }
func (s *stubLTCommand) Signal(os.Signal) error { return nil }
func (s *stubLTCommand) Kill() error            { return nil }
func (s *stubLTCommand) StdoutPipe() (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		fmt.Fprintln(pw, s.output)
		pw.Close()
	}()
	return pr, nil
}

func TestStartLocaltunnel(t *testing.T) {
	originalFactory := localtunnelFactory
	localtunnelFactory = func(ctx context.Context, host string, port string) (ltCommand, error) {
		if port != "8080" {
			t.Fatalf("expected port 8080, got %s", port)
		}
		return &stubLTCommand{output: "your url is: https://lt.dev"}, nil
	}
	defer func() { localtunnelFactory = originalFactory }()

	ctx := context.Background()
	session, err := startLocaltunnel(ctx, tunnelOptions{
		Provider:   "localtunnel",
		ListenAddr: "127.0.0.1:8080",
	})
	if err != nil {
		t.Fatalf("startLocaltunnel: %v", err)
	}
	if session == nil || session.URL != "https://lt.dev" {
		t.Fatalf("unexpected session: %#v", session)
	}
	if err := session.Close(context.Background()); err != nil {
		t.Fatalf("close localtunnel: %v", err)
	}
}

func TestResolveTunnelProviderAutoPreference(t *testing.T) {
	originalLookPath := lookPath
	defer func() { lookPath = originalLookPath }()
	lookPath = func(file string) (string, error) {
		if file == "ngrok" {
			return "/usr/bin/ngrok", nil
		}
		return "", os.ErrNotExist
	}
	provider, err := resolveTunnelProvider("auto")
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if provider != "ngrok" {
		t.Fatalf("expected ngrok, got %s", provider)
	}
	lookPath = func(file string) (string, error) {
		if file == "lt" {
			return "/usr/local/bin/lt", nil
		}
		return "", os.ErrNotExist
	}
	provider, err = resolveTunnelProvider("auto")
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if provider != "localtunnel" {
		t.Fatalf("expected localtunnel fallback, got %s", provider)
	}
}

func TestStartNgrokTunnelTimeoutKillsProcess(t *testing.T) {
	fake := &fakeProcess{}
	originalFactory := processFactory
	processFactory = func(ctx context.Context, bin string, args []string, env []string) (process, error) {
		return fake, nil
	}
	defer func() { processFactory = originalFactory }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tunnels":[]}`))
	}))
	defer server.Close()

	originalClient := httpClient
	httpClient = server.Client()
	defer func() { httpClient = originalClient }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := startNgrokTunnel(ctx, tunnelOptions{
		Provider:   "ngrok",
		ListenAddr: "127.0.0.1:8080",
		NgrokAPI:   server.URL,
	})
	if err == nil {
		t.Fatalf("expected error when ngrok URL never becomes available")
	}
	if atomic.LoadInt32(&fake.kills) == 0 {
		t.Fatalf("expected ngrok process kill when tunnel never becomes ready")
	}
}

type trackingLTCommand struct {
	output string
	kills  int32
}

func (s *trackingLTCommand) Start() error           { return nil }
func (s *trackingLTCommand) Wait() error            { return nil }
func (s *trackingLTCommand) Signal(os.Signal) error { return nil }
func (s *trackingLTCommand) Kill() error {
	atomic.AddInt32(&s.kills, 1)
	return nil
}
func (s *trackingLTCommand) StdoutPipe() (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		fmt.Fprintln(pw, s.output)
		pw.Close()
	}()
	return pr, nil
}

func TestStartLocaltunnelKillOnURLFailure(t *testing.T) {
	cmd := &trackingLTCommand{output: "waiting..."}
	originalFactory := localtunnelFactory
	localtunnelFactory = func(ctx context.Context, host string, port string) (ltCommand, error) {
		return cmd, nil
	}
	defer func() { localtunnelFactory = originalFactory }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, err := startLocaltunnel(ctx, tunnelOptions{
		Provider:   "localtunnel",
		ListenAddr: "127.0.0.1:8080",
	})
	if err == nil || !strings.Contains(err.Error(), "localtunnel exited") {
		t.Fatalf("expected localtunnel exit error, got %v", err)
	}
	if atomic.LoadInt32(&cmd.kills) == 0 {
		t.Fatalf("expected localtunnel process kill when no URL produced")
	}
}

func TestStartTunnelAutoFallbacksToLocaltunnel(t *testing.T) {
	originalProcessFactory := processFactory
	processFactory = func(ctx context.Context, bin string, args []string, env []string) (process, error) {
		return nil, fmt.Errorf("missing %s binary", bin)
	}
	defer func() { processFactory = originalProcessFactory }()

	originalLTFactory := localtunnelFactory
	localtunnelFactory = func(ctx context.Context, host string, port string) (ltCommand, error) {
		return &stubLTCommand{output: "https://fallback.localtunnel.me"}, nil
	}
	defer func() { localtunnelFactory = originalLTFactory }()

	session, err := startTunnel(context.Background(), tunnelOptions{
		Provider:   "auto",
		ListenAddr: "127.0.0.1:8080",
	})
	if err != nil {
		t.Fatalf("startTunnel auto: %v", err)
	}
	if session == nil || session.Provider != "localtunnel" || session.URL != "https://fallback.localtunnel.me" {
		t.Fatalf("expected localtunnel fallback, got %#v", session)
	}
}

func TestResolveTunnelProviderAutoNoBinary(t *testing.T) {
	originalLookPath := lookPath
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	defer func() { lookPath = originalLookPath }()

	_, err := resolveTunnelProvider("auto")
	if err == nil || !strings.Contains(err.Error(), "no supported tunnel binary") {
		t.Fatalf("expected helpful error when auto provider unavailable, got %v", err)
	}
}

type blockingProcess struct {
	waitCh  chan struct{}
	signals int32
	kills   int32
}

func newBlockingProcess() *blockingProcess {
	return &blockingProcess{waitCh: make(chan struct{})}
}

func (p *blockingProcess) Start() error { return nil }
func (p *blockingProcess) Wait() error {
	<-p.waitCh
	return nil
}
func (p *blockingProcess) Signal(os.Signal) error {
	atomic.AddInt32(&p.signals, 1)
	return nil
}
func (p *blockingProcess) Kill() error {
	atomic.AddInt32(&p.kills, 1)
	close(p.waitCh)
	return nil
}

func TestTunnelSessionCloseForcesKillWhenContextExpires(t *testing.T) {
	blocking := newBlockingProcess()
	originalFactory := processFactory
	processFactory = func(ctx context.Context, bin string, args []string, env []string) (process, error) {
		return blocking, nil
	}
	defer func() { processFactory = originalFactory }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tunnels":[{"public_url":"https://cli.ngrok.dev"}]}`))
	}))
	defer server.Close()

	originalClient := httpClient
	httpClient = server.Client()
	defer func() { httpClient = originalClient }()

	session, err := startNgrokTunnel(context.Background(), tunnelOptions{
		Provider:   "ngrok",
		ListenAddr: "127.0.0.1:8080",
		NgrokAPI:   server.URL,
	})
	if err != nil {
		t.Fatalf("startNgrokTunnel: %v", err)
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := session.Close(closeCtx); err != nil {
		t.Fatalf("close TunnelSession: %v", err)
	}
	if atomic.LoadInt32(&blocking.kills) == 0 {
		t.Fatalf("expected kill when tunnel session fails to shut down in time")
	}
	if atomic.LoadInt32(&blocking.signals) == 0 {
		t.Fatalf("expected graceful signal before kill")
	}
}
