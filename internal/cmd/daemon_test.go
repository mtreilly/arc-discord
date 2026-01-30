package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDaemonManagerStartWritesPID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")
	startProcess = func(ctx context.Context, argv []string, logPath, workdir string, env []string) (int, error) {
		return 1234, nil
	}
	checkProcess = func(pid int) bool { return false }
	defer func() {
		startProcess = realStartProcess
		checkProcess = realCheckProcess
	}()
	m := newDaemonManager(daemonOptions{PIDFile: pidPath})
	if err := m.Start(context.Background(), []string{"/bin/echo"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid, err := readPID(pidPath)
	if err != nil {
		t.Fatalf("readPID: %v", err)
	}
	if pid != 1234 {
		t.Fatalf("expected pid 1234, got %d", pid)
	}
}

func TestDaemonManagerStopKillsProcess(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")
	if err := writePID(pidPath, 4321); err != nil {
		t.Fatalf("writePID: %v", err)
	}
	killed := false
	killProcess = func(pid int) error {
		killed = true
		if pid != 4321 {
			return errors.New("wrong pid")
		}
		return nil
	}
	defer func() { killProcess = realKillProcess }()
	m := newDaemonManager(daemonOptions{PIDFile: pidPath})
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !killed {
		t.Fatalf("expected kill to be invoked")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("pid file should be removed")
	}
}

func TestDaemonManagerStatus(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")
	if err := writePID(pidPath, 99); err != nil {
		t.Fatalf("writePID: %v", err)
	}
	checkProcess = func(pid int) bool { return pid == 99 }
	defer func() { checkProcess = realCheckProcess }()
	m := newDaemonManager(daemonOptions{PIDFile: pidPath})
	status, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status == "stopped" {
		t.Fatalf("expected running status")
	}
}

func TestEnvFromFileParsesLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env")
	content := "# comment\nFOO=bar\nBAZ=qux\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	env := envFromFile(path)
	if len(env) != 2 || env[0] != "FOO=bar" || env[1] != "BAZ=qux" {
		t.Fatalf("unexpected env %v", env)
	}
}
