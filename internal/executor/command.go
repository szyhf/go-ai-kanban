package executor

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

// BuildCommand creates an exec.Cmd with process group settings for clean kill.
// On Unix, uses Setpgid to create a new process group.
func BuildCommand(dir string, name string, args []string, env []string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	// Create a new process group so we can kill the entire group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, nil
}

// KillProcessGroup kills the entire process group associated with cmd.
// This ensures child processes (like MCP servers) are also terminated.
func KillProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return fmt.Errorf("process not started")
	}
	// Kill the entire process group by using negative PID.
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// InterruptProcessGroup sends SIGINT to the process group for graceful termination.
func InterruptProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return fmt.Errorf("process not started")
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
}

// WaitProcess waits for the process to exit and returns the exit code.
// It should be called in a goroutine.
func WaitProcess(ctx context.Context, cmd *exec.Cmd, done chan<- ExitResult) {
	err := cmd.Wait()
	select {
	case <-ctx.Done():
		return
	case done <- ExitResultFromErr(err):
	}
}

// ExitResultFromErr maps a cmd.Wait() error to an ExitResult.
func ExitResultFromErr(err error) ExitResult {
	if err == nil {
		return ExitResult{Code: 0}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return ExitResult{Code: exitErr.ExitCode(), Error: err}
	}
	return ExitResult{Code: -1, Error: err}
}
