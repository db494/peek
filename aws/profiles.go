// Package aws handles shared-config profile discovery and EC2 API access.
package aws

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/ini.v1"
)

// Profiles returns the names of all AWS profiles found in ~/.aws/config and
// ~/.aws/credentials, deduplicated and sorted. The AWS_CONFIG_FILE and
// AWS_SHARED_CREDENTIALS_FILE environment variables are honored.
func Profiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := os.Getenv("AWS_CONFIG_FILE")
	if configPath == "" {
		configPath = filepath.Join(home, ".aws", "config")
	}
	credsPath := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credsPath == "" {
		credsPath = filepath.Join(home, ".aws", "credentials")
	}

	seen := map[string]bool{}

	// In the config file, profile sections are named "profile <name>",
	// except for "default" which appears bare.
	for _, name := range sectionNames(configPath) {
		if name == "default" {
			seen["default"] = true
		} else if rest, ok := strings.CutPrefix(name, "profile "); ok {
			seen[strings.TrimSpace(rest)] = true
		}
	}

	// In the credentials file, sections are the profile names themselves.
	for _, name := range sectionNames(credsPath) {
		seen[name] = true
	}

	profiles := make([]string, 0, len(seen))
	for p := range seen {
		profiles = append(profiles, p)
	}
	sort.Strings(profiles)
	return profiles, nil
}

// ProfileRegion returns the region configured for the given profile in
// ~/.aws/config, or "" if none is set.
func ProfileRegion(profile string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	configPath := os.Getenv("AWS_CONFIG_FILE")
	if configPath == "" {
		configPath = filepath.Join(home, ".aws", "config")
	}
	cfg, err := ini.Load(configPath)
	if err != nil {
		return ""
	}
	sectionName := "profile " + profile
	if profile == "default" {
		sectionName = "default"
	}
	section, err := cfg.GetSection(sectionName)
	if err != nil {
		return ""
	}
	return section.Key("region").String()
}

// sectionNames returns the section names of an INI file, or nil if the file
// is missing or unparseable.
func sectionNames(path string) []string {
	cfg, err := ini.Load(path)
	if err != nil {
		return nil
	}
	var names []string
	for _, s := range cfg.Sections() {
		if s.Name() == ini.DefaultSection {
			continue
		}
		names = append(names, s.Name())
	}
	return names
}
