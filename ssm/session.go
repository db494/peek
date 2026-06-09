// Package ssm hands the terminal over to an interactive SSM session.
package ssm

import (
	"fmt"
	"os"
	"os/exec"
)

// StartSession runs `aws ssm start-session --target <instanceID>` with the
// given profile and region, attaching the process to the user's terminal.
// It must only be called after the TUI has fully exited so the terminal is
// back in its normal state. The returned error preserves the session's exit
// code as an *exec.ExitError.
func StartSession(profile, region, instanceID string) error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("the aws CLI was not found in PATH — install AWS CLI v2 to use SSM sessions")
	}

	args := []string{"ssm", "start-session", "--target", instanceID}
	if region != "" {
		args = append(args, "--region", region)
	}
	if profile != "" {
		args = append(args, "--profile", profile)
	}

	cmd := exec.Command("aws", args...)
	cmd.Env = os.Environ()
	if profile != "" {
		cmd.Env = append(cmd.Env, "AWS_PROFILE="+profile)
	}
	if region != "" {
		cmd.Env = append(cmd.Env, "AWS_REGION="+region)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
