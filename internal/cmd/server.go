package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/discord/interactions"
	arcer "github.com/yourorg/arc-sdk/errors"
)

var newDaemonManagerFn = func(opts daemonOptions) daemonController { return newDaemonManager(opts) }
var newRedisPublisherFn = newRedisPublisher

func serverCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the Discord interactions HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(serverStartCmd(opts))
	cmd.AddCommand(serverStopCmd())
	cmd.AddCommand(serverStatusCmd())
	return cmd
}

func serverStartCmd(opts *globalOptions) *cobra.Command {
	var (
		listenAddr     string
		publicURL      string
		redisAddr      string
		redisDB        int
		redisPass      string
		redisPrefix    string
		dryRun         bool
		tunnelProvider string
		ngrokToken     string
		daemonEnabled  bool
		pidFile        string
		logFile        string
		workdir        string
		envFile        string
		checkPrereqs   bool
		showExample    bool
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the HTTP server that receives Discord interactions",
		RunE: func(cmd *cobra.Command, args []string) error {
			startOpts := serverStartOptions{
				ListenAddr:     listenAddr,
				PublicURL:      publicURL,
				RedisAddr:      redisAddr,
				RedisDB:        redisDB,
				RedisPass:      redisPass,
				RedisPrefix:    redisPrefix,
				TunnelProvider: tunnelProvider,
				NgrokToken:     ngrokToken,
				DryRun:         dryRun,
				Daemon:         daemonEnabled,
				DaemonOpts: daemonOptions{
					PIDFile: pidFile,
					LogFile: logFile,
					Workdir: workdir,
					EnvFile: envFile,
				},
			}

			// Handle --example flag
			if showExample {
				cmd.Println(GenerateExampleConfig())
				return nil
			}

			// Handle --check-prereqs flag or run prereq check before starting
			checker := NewServerPrereqChecker(opts, startOpts)
			report, err := checker.Check(cmd.Context())
			if err != nil {
				return err
			}

			if checkPrereqs {
				cmd.Println(report.FormatReport())
				if !report.AllPassed {
					return &arcer.CLIError{
						Msg:  "Prerequisites check failed",
						Hint: "Fix the issues above and try again",
					}
				}
				return nil
			}

			// If prerequisites failed, show helpful error
			if !report.AllPassed {
				cmd.Println(report.FormatReport())
				return &arcer.CLIError{
					Msg:  "Cannot start server: prerequisites not met",
					Hint: "Fix the issues above and try again, or run with --check-prereqs for more details",
				}
			}

			return runServerStart(cmd, opts, startOpts)
		},
		Example: `  # Check what's needed before starting
  arc-discord server start --check-prereqs

  # Show example configuration
  arc-discord server start --example

  # Start the server on default settings
  arc-discord server start

  # Bind to a specific port and redis instance
  arc-discord server start --listen :9090 --redis-addr redis.internal:6379

  # Development mode with ngrok tunnel
  arc-discord server start --tunnel ngrok --ngrok-auth-token $NGROK_TOKEN

  # Auto-detect tunnel provider (ngrok preferred, otherwise localtunnel)
  arc-discord server start --tunnel auto

  # Run as a background daemon with PID/log files
  arc-discord server start --daemon --pid-file /tmp/discord.pid --log-file /tmp/discord.log

  # Skip signature verification (development only)
  arc-discord server start --dry-run`,
	}

	// Setup and diagnostics flags
	cmd.Flags().BoolVar(&checkPrereqs, "check-prereqs", false, "Check prerequisites and show setup instructions")
	cmd.Flags().BoolVar(&showExample, "example", false, "Show example configuration file")

	// Server configuration flags
	cmd.Flags().StringVar(&listenAddr, "listen", "", "HTTP listen address (overrides server.listen_addr)")
	cmd.Flags().StringVar(&publicURL, "public-url", "", "Public URL that Discord will hit (optional override)")

	// Redis flags
	cmd.Flags().StringVar(&redisAddr, "redis-addr", "", "Redis address for publishing events")
	cmd.Flags().IntVar(&redisDB, "redis-db", 0, "Redis database index")
	cmd.Flags().StringVar(&redisPass, "redis-password", "", "Redis password")
	cmd.Flags().StringVar(&redisPrefix, "redis-prefix", "", "Redis channel prefix (default arc:discord)")

	// Tunnel flags
	cmd.Flags().StringVar(&tunnelProvider, "tunnel", "", "Enable a development tunnel: ngrok|localtunnel|auto")
	cmd.Flags().StringVar(&ngrokToken, "ngrok-auth-token", "", "Ngrok auth token (overrides tunnel.ngrok_auth_token)")

	// Development flags
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Skip signature verification (development only)")

	// Daemon flags
	cmd.Flags().BoolVar(&daemonEnabled, "daemon", false, "Run the server in the background")
	cmd.Flags().StringVar(&pidFile, "pid-file", "", "PID file path for daemon mode (default ~/.cache/arc/discord-server.pid)")
	cmd.Flags().StringVar(&logFile, "log-file", "", "Log file for daemon stdout/stderr")
	cmd.Flags().StringVar(&workdir, "workdir", "", "Working directory for daemonized server")
	cmd.Flags().StringVar(&envFile, "env-file", "", "Optional env file (KEY=value per line) for daemon mode")

	return cmd
}

type serverStartOptions struct {
	ListenAddr     string
	PublicURL      string
	RedisAddr      string
	RedisDB        int
	RedisPass      string
	RedisPrefix    string
	DryRun         bool
	TunnelProvider string
	NgrokToken     string
	Daemon         bool
	DaemonOpts     daemonOptions
}

func runServerStart(cmd *cobra.Command, opts *globalOptions, overrides serverStartOptions) error {
	if overrides.Daemon && os.Getenv(daemonEnvFlag) == "" {
		execPath, err := os.Executable()
		if err != nil {
			return err
		}
		argv := filterDaemonArgv(execPath, os.Args)
		mgr := newDaemonManagerFn(overrides.DaemonOpts)
		if err := mgr.Start(cmd.Context(), argv); err != nil {
			return err
		}
		cmd.Printf("daemon started (pid file %s)\n", mgr.PIDPath())
		return nil
	}
	_, extra, cfgPath, err := opts.loadConfigWithInteractions()
	if err != nil {
		return err
	}
	if overrides.ListenAddr != "" {
		extra.Server.ListenAddr = overrides.ListenAddr
	}
	if overrides.PublicURL != "" {
		extra.PublicURL = overrides.PublicURL
	}
	if overrides.RedisAddr != "" {
		extra.Redis.Addr = overrides.RedisAddr
	}
	if overrides.RedisPass != "" {
		extra.Redis.Password = overrides.RedisPass
	}
	if overrides.RedisDB != 0 {
		extra.Redis.DB = overrides.RedisDB
	}
	if overrides.RedisPrefix != "" {
		extra.Redis.ChannelPrefix = overrides.RedisPrefix
	}
	if overrides.TunnelProvider != "" {
		extra.Tunnel.Provider = overrides.TunnelProvider
	}
	if overrides.NgrokToken != "" {
		extra.Tunnel.NgrokAuthToken = overrides.NgrokToken
	}
	if extra.PublicKey == "" {
		return &arcer.CLIError{Msg: "discord.public_key is required for signature verification"}
	}

	publisher, err := newRedisPublisherFn(extra.Redis)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to connect to redis"}).WithCause(err)
	}
	defer publisher.Close()

	serverOptions := []interactions.ServerOption{}
	if overrides.DryRun {
		serverOptions = append(serverOptions, interactions.WithDryRun(true))
	}
	srv, err := interactions.NewServer(extra.PublicKey, serverOptions...)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize interaction server"}).WithCause(err)
	}

	bindings := collectHandlerBindings(extra.Interactions)
	if err := registerInteractionHandlers(srv, extra.Interactions.Timeout, publisher, bindings); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/interactions", srv.HandleInteraction)

	tunnelSession, err := maybeStartTunnel(cmd.Context(), cmd, extra, overrides)
	if err != nil {
		return err
	}
	defer func() {
		if tunnelSession != nil {
			_ = tunnelSession.Close(context.Background())
		}
	}()

	httpServer := &http.Server{
		Addr:    extra.Server.ListenAddr,
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		cmd.Printf("Discord interaction server listening on %s (config: %s)\n", extra.Server.ListenAddr, cfgPath)
		if extra.PublicURL != "" {
			cmd.Printf("Public URL: %s\n", extra.PublicURL)
		}
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		cmd.Println("Discord interaction server stopped")
		return nil
	case err := <-errCh:
		if err != nil {
			return (&arcer.CLIError{Msg: "interaction server exited with error"}).WithCause(err)
		}
		return nil
	}
}

func maybeStartTunnel(ctx context.Context, cmd *cobra.Command, cfg *interactionSettings, overrides serverStartOptions) (*TunnelSession, error) {
	provider, err := resolveTunnelProvider(cfg.Tunnel.Provider)
	if err != nil {
		return nil, (&arcer.CLIError{Msg: "unable to determine tunnel provider"}).WithCause(err)
	}
	if provider == "" {
		return nil, nil
	}
	cfg.Tunnel.Provider = provider
	session, err := startTunnel(ctx, tunnelOptions{
		Provider:       provider,
		ListenAddr:     cfg.Server.ListenAddr,
		NgrokAuthToken: cfg.Tunnel.NgrokAuthToken,
	})
	if err != nil {
		return nil, (&arcer.CLIError{Msg: fmt.Sprintf("failed to start %s tunnel", provider)}).WithCause(err)
	}
	cfg.PublicURL = session.URL
	cmd.Printf("Tunnel (%s) ready: %s\n", provider, session.URL)
	return session, nil
}

func filterDaemonArgv(execPath string, args []string) []string {
	var filtered []string
	for i := 1; i < len(args); i++ {
		if args[i] == "--daemon" {
			continue
		}
		filtered = append(filtered, args[i])
	}
	return append([]string{execPath}, filtered...)
}

func serverStopCmd() *cobra.Command {
	var pidFile string
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Discord interaction server daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := newDaemonManagerFn(daemonOptions{PIDFile: pidFile})
			if err := mgr.Stop(cmd.Context()); err != nil {
				return err
			}
			cmd.Println("daemon stopped")
			return nil
		},
	}
	cmd.Flags().StringVar(&pidFile, "pid-file", "", "PID file path (default ~/.cache/arc/discord-server.pid)")
	return cmd
}

func serverStatusCmd() *cobra.Command {
	var pidFile string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := newDaemonManagerFn(daemonOptions{PIDFile: pidFile})
			status, err := mgr.Status()
			if err != nil {
				return err
			}
			cmd.Println(status)
			return nil
		},
	}
	cmd.Flags().StringVar(&pidFile, "pid-file", "", "PID file path (default ~/.cache/arc/discord-server.pid)")
	return cmd
}
