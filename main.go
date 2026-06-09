// peek is an interactive EC2 instance browser that hands off to an AWS SSM
// session for the selected instance.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"peek/aws"
	"peek/ssm"
	"peek/tui"
)

func main() {
	profile := flag.String("profile", "", "AWS profile to use (skips the interactive picker)")
	region := flag.String("region", "", "AWS region (defaults to the profile's configured region)")
	demo := flag.Bool("demo", false, "run with fake data instead of calling AWS")
	flag.Parse()

	code, err := run(*profile, *region, *demo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(code)
}

func run(profile, region string, demo bool) (int, error) {
	cfg := tui.Config{Profile: profile, Region: region, Demo: demo}

	if !demo {
		profiles, err := aws.Profiles()
		if err != nil {
			return 1, fmt.Errorf("detecting AWS profiles: %w", err)
		}
		if len(profiles) == 0 && profile == "" {
			return 1, errors.New("no AWS profiles found in ~/.aws/config or ~/.aws/credentials")
		}
		cfg.Profiles = profiles

		// Skip the profile picker when the choice is already unambiguous.
		if cfg.Profile == "" {
			if env := os.Getenv("AWS_PROFILE"); env != "" {
				cfg.Profile = env
			} else if len(profiles) == 1 {
				cfg.Profile = profiles[0]
			}
		}
	}

	// The alt screen is restored by Bubble Tea before Run returns (even on
	// panic), so the SSM handoff below starts from a clean terminal.
	p := tea.NewProgram(tui.New(cfg), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return 1, fmt.Errorf("running TUI: %w", err)
	}

	model, ok := final.(tui.Model)
	if !ok || model.Selected == nil {
		return 0, nil // user quit without selecting an instance
	}

	if demo {
		fmt.Printf("demo mode: would run `aws ssm start-session --target %s` (profile %s, region %s)\n",
			model.Selected.ID, model.Profile(), model.Region())
		return 0, nil
	}

	fmt.Printf("Starting SSM session with %s (%s) in %s…\n",
		model.Selected.ID, model.Selected.Name, model.Region())

	if err := ssm.StartSession(model.Profile(), model.Region(), model.Selected.ID); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil // session's own diagnostics already went to stderr
		}
		return 1, err
	}

	fmt.Println("SSM session ended.")
	return 0, nil
}
