package cmd

import (
	"context"
	"errors"
	"testing"
)

type fakeDaemon struct {
	startArgs []string
	started   bool
	stopped   bool
	status    string
	startErr  error
	stopErr   error
	statusErr error
}

func (f *fakeDaemon) Start(ctx context.Context, argv []string) error {
	f.started = true
	f.startArgs = argv
	return f.startErr
}

func (f *fakeDaemon) Stop(ctx context.Context) error {
	f.stopped = true
	return f.stopErr
}

func (f *fakeDaemon) Status() (string, error) {
	if f.statusErr != nil {
		return "", f.statusErr
	}
	return f.status, nil
}

func (f *fakeDaemon) PIDPath() string { return "pid" }

func TestServerStopCmdUsesDaemonManager(t *testing.T) {
	stub := &fakeDaemon{}
	newDaemonManagerFn = func(opts daemonOptions) daemonController { return stub }
	t.Cleanup(func() {
		newDaemonManagerFn = func(opts daemonOptions) daemonController { return newDaemonManager(opts) }
	})

	cmd := serverStopCmd()
	cmd.Flags().Set("pid-file", "pid")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stop execute: %v", err)
	}
	if !stub.stopped {
		t.Fatalf("expected daemon Stop to be called")
	}
}

func TestServerStatusCmdUsesDaemonManager(t *testing.T) {
	stub := &fakeDaemon{status: "running"}
	newDaemonManagerFn = func(opts daemonOptions) daemonController { return stub }
	t.Cleanup(func() {
		newDaemonManagerFn = func(opts daemonOptions) daemonController { return newDaemonManager(opts) }
	})
	cmd := serverStatusCmd()
	cmd.Flags().Set("pid-file", "pid")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status execute: %v", err)
	}
	if !stub.started && stub.status == "" {
		t.Fatalf("expected status to be queried")
	}
}

func TestServerStatusCmdPropagatesError(t *testing.T) {
	stub := &fakeDaemon{statusErr: errors.New("boom")}
	newDaemonManagerFn = func(opts daemonOptions) daemonController { return stub }
	t.Cleanup(func() {
		newDaemonManagerFn = func(opts daemonOptions) daemonController { return newDaemonManager(opts) }
	})
	cmd := serverStatusCmd()
	cmd.Flags().Set("pid-file", "pid")
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error from status")
	}
}
