package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/db494/peek/internal/tui"
)

var allowedProfiles = []string{
	"default",
	"dev",
	"staging",
	"prod",
}

func main() {
	var profileVal string
	flag.StringVar(&profileVal, "profile", "", "AWS profile name")
	flag.StringVar(&profileVal, "p", "", "AWS profile name (shorthand)")
	region := flag.String("region", "", "AWS region")
	flag.Parse()

	if profileVal != "" {
		allowed := false
		for _, p := range allowedProfiles {
			if p == profileVal {
				allowed = true
				break
			}
		}
		if !allowed {
			fmt.Fprintf(os.Stderr, "profile %q does not exist\n", profileVal)
			os.Exit(1)
		}
	}

	ctx := context.Background()

	var opts []func(*config.LoadOptions) error
	if profileVal != "" {
		opts = append(opts, config.WithSharedConfigProfile(profileVal))
	}
	if *region != "" {
		opts = append(opts, config.WithRegion(*region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading AWS config: %v\n", err)
		os.Exit(1)
	}

	selected, err := tui.Run(ctx, cfg, profileVal)
	if err == tui.ErrNoInstances {
		fmt.Fprintln(os.Stderr, "no instances found")
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if selected == nil {
		os.Exit(0)
	}

	if err := startSSMSession(ctx, cfg, selected.ID, profileVal, *region); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func startSSMSession(ctx context.Context, cfg aws.Config, instanceID, profile, region string) error {
	pluginPath, err := exec.LookPath("session-manager-plugin")
	if err != nil {
		return fmt.Errorf("session-manager-plugin not found in PATH: %w", err)
	}

	client := ssm.NewFromConfig(cfg)
	resp, err := client.StartSession(ctx, &ssm.StartSessionInput{
		Target: aws.String(instanceID),
	})
	if err != nil {
		return fmt.Errorf("StartSession: %w", err)
	}

	sessionJSON, err := json.Marshal(map[string]string{
		"SessionId":  aws.ToString(resp.SessionId),
		"TokenValue": aws.ToString(resp.TokenValue),
		"StreamUrl":  aws.ToString(resp.StreamUrl),
	})
	if err != nil {
		return err
	}

	paramsJSON, err := json.Marshal(map[string]string{"Target": instanceID})
	if err != nil {
		return err
	}

	if region == "" {
		region = cfg.Region
	}

	endpoint := "https://ssm." + region + ".amazonaws.com"

	return syscall.Exec(pluginPath, []string{
		"session-manager-plugin",
		string(sessionJSON),
		region,
		"StartSession",
		profile,
		string(paramsJSON),
		endpoint,
	}, os.Environ())
}
