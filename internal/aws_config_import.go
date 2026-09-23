package internal

import (
	"os"
	"sort"
	"strings"

	"gopkg.in/ini.v1"
)

// AwsConfigFilePath returns the path to the AWS CLI's own config file, respecting the
// AWS_CONFIG_FILE environment variable like the AWS CLI/SDKs do, defaulting to ~/.aws/config.
func AwsConfigFilePath() string {
	if v := os.Getenv("AWS_CONFIG_FILE"); v != "" {
		return v
	}
	homeDir, _ := os.UserHomeDir()
	return homeDir + "/.aws/config"
}

// ImportSsoInstancesFromAwsConfig scans the AWS CLI's config file for SSO organizations and
// returns them as SsoInstances, deduplicated by start-url. Recognizes both the modern
// `[sso-session NAME]` shape and the legacy inline `sso_start_url` on a `[profile NAME]` (or
// `[default]`) section. A missing file is not an error - it just yields no results.
func ImportSsoInstancesFromAwsConfig(path string) ([]SsoInstance, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}
	cfg, err := ini.Load(path)
	if err != nil {
		return nil, err
	}

	var discovered []SsoInstance
	seen := map[string]bool{}
	add := func(name string, startUrl string, region string) {
		if startUrl == "" || seen[startUrl] {
			return
		}
		seen[startUrl] = true
		discovered = append(discovered, SsoInstance{Name: name, StartUrl: startUrl, Region: region})
	}

	for _, section := range cfg.Sections() {
		name := section.Name()
		switch {
		case strings.HasPrefix(name, "sso-session "):
			add(strings.TrimPrefix(name, "sso-session "),
				section.Key("sso_start_url").String(), section.Key("sso_region").String())
		case strings.HasPrefix(name, "profile ") && section.HasKey("sso_start_url"):
			add(strings.TrimPrefix(name, "profile "),
				section.Key("sso_start_url").String(), section.Key("sso_region").String())
		case name == "default" && section.HasKey("sso_start_url"):
			add("default", section.Key("sso_start_url").String(), section.Key("sso_region").String())
		}
	}

	sort.Slice(discovered, func(i, j int) bool { return discovered[i].StartUrl < discovered[j].StartUrl })
	return discovered, nil
}
