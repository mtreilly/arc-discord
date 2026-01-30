package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

const daemonEnvFlag = "VIBE_DISCORD_DAEMON_CHILD"

type daemonOptions struct {
	PIDFile string
	LogFile string
	Workdir string
	EnvFile string
}

type daemonManager struct {
	opts        daemonOptions
	startProc   startProcessFn
	checkProc   checkProcessFn
	killProc    killProcessFn
	openLogFile func(path string) (io.WriteCloser, error)
}

type daemonController interface {
	Start(ctx context.Context, argv []string) error
	Stop(ctx context.Context) error
	Status() (string, error)
	PIDPath() string
}

type startProcessFn func(ctx context.Context, argv []string, logPath, workdir string, extraEnv []string) (int, error)
type checkProcessFn func(pid int) bool
type killProcessFn func(pid int) error

var (
	startProcess = realStartProcess
	checkProcess = realCheckProcess
	killProcess  = realKillProcess
)

func newDaemonManager(opts daemonOptions) *daemonManager {
	return &daemonManager{
		opts:      opts,
		startProc: startProcess,
		checkProc: checkProcess,
		killProc:  killProcess,
		openLogFile: func(path string) (io.WriteCloser, error) {
			if path == "" {
				return nil, nil
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		},
	}
}

func (m *daemonManager) Start(ctx context.Context, argv []string) error {
	if m.opts.PIDFile == "" {
		m.opts.PIDFile = defaultPIDPath()
	}
	if pid, _ := readPID(m.opts.PIDFile); pid > 0 && m.checkProc(pid) {
		return fmt.Errorf("daemon already running (pid %d)", pid)
	}

	env := append(envFromFile(m.opts.EnvFile), fmt.Sprintf("%s=1", daemonEnvFlag))
	pid, err := m.startProc(ctx, argv, m.opts.LogFile, m.opts.Workdir, env)
	if err != nil {
		return err
	}
	if err := writePID(m.opts.PIDFile, pid); err != nil {
		return err
	}
	return nil
}

func (m *daemonManager) Stop(ctx context.Context) error {
	pid, err := readPID(m.opts.PIDFile)
	if err != nil {
		return err
	}
	if pid == 0 {
		return errors.New("no pid file found")
	}
	if err := m.killProc(pid); err != nil {
		return err
	}
	_ = os.Remove(m.opts.PIDFile)
	return nil
}

func (m *daemonManager) Status() (string, error) {
	pid, err := readPID(m.opts.PIDFile)
	if err != nil {
		return "unknown", err
	}
	if pid == 0 {
		return "stopped", nil
	}
	if m.checkProc(pid) {
		return fmt.Sprintf("running (pid %d)", pid), nil
	}
	return fmt.Sprintf("stale pid file (%d)", pid), nil
}

func (m *daemonManager) PIDPath() string {
	return m.opts.PIDFile
}

func defaultPIDPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "vibe", "discord-server.pid")
}

func envFromFile(path string) []string {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var env []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		env = append(env, line)
	}
	return env
}

func writePID(path string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d", pid)), 0o644)
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, err
	}
	return pid, nil
}

func realStartProcess(ctx context.Context, argv []string, logPath, workdir string, extraEnv []string) (int, error) {
	if len(argv) == 0 {
		return 0, errors.New("missing argv for daemon start")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if workdir != "" {
		cmd.Dir = workdir
	}
	if logPath != "" {
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return 0, err
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	cmd.Env = append(os.Environ(), extraEnv...)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func realCheckProcess(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, Signal 0 checks existence.
	if runtime.GOOS == "windows" {
		return true
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func realKillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return proc.Kill()
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	return nil
}
