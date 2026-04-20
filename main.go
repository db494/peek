package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/db494/peek/internal/tui"
)

func loadAWSProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}

	seen := map[string]struct{}{"default": {}}

	parseINI := func(path string, prefixed bool) error {
		f, err := os.Open(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}

		defer f.Close() //nolint:errcheck

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
				continue
			}
			section := line[1 : len(line)-1]
			if section == "default" {
				continue
			}
			if prefixed {
				if !strings.HasPrefix(section, "profile ") {
					continue
				}
				section = strings.TrimPrefix(section, "profile ")
			}
			section = strings.TrimSpace(section)
			if section != "" {
				seen[section] = struct{}{}
			}
		}
		return scanner.Err()
	}

	if err := parseINI(filepath.Join(home, ".aws", "config"), true); err != nil {
		return nil, fmt.Errorf("reading ~/.aws/config: %w", err)
	}
	if err := parseINI(filepath.Join(home, ".aws", "credentials"), false); err != nil {
		return nil, fmt.Errorf("reading ~/.aws/credentials: %w", err)
	}

	profiles := make([]string, 0, len(seen))
	for p := range seen {
		profiles = append(profiles, p)
	}
	sort.Strings(profiles)
	return profiles, nil
}

func main() {
	var profileVal string
	flag.StringVar(&profileVal, "profile", "", "AWS profile name")
	flag.StringVar(&profileVal, "p", "", "AWS profile name (shorthand)")
	region := flag.String("region", "", "AWS region")
	listProfiles := flag.Bool("list-profiles", false, "List available AWS profiles in config file")
	flag.Parse()

	profiles, err := loadAWSProfiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load AWS profiles: %v\n", err)
		profiles = []string{"default"}
	}

	if *listProfiles {
		fmt.Println(strings.Join(profiles, "\n"))
		os.Exit(0)
	}

	if profileVal != "" && !slices.Contains(profiles, profileVal) {
		fmt.Fprintf(os.Stderr, "profile %q not found in ~/.aws/config or ~/.aws/credentials\n", profileVal)
		fmt.Fprintf(os.Stderr, "available profiles: %s\n", strings.Join(profiles, ", "))
		os.Exit(1)
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
