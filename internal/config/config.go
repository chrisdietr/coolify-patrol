package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"

	"github.com/chrisdietr/coolify-patrol/pkg/types"
)

var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// Load reads and parses the configuration, supporting both YAML files and environment variables.
// Environment variables take precedence over YAML configuration.
func Load(path string) (*types.Config, error) {
	var config types.Config
	
	// Try to load YAML file first (optional)
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			// Substitute environment variables in YAML
			expanded := envVarRegex.ReplaceAllStringFunc(string(data), func(match string) string {
				varName := envVarRegex.FindStringSubmatch(match)[1]
				if value := os.Getenv(varName); value != "" {
					return value
				}
				return match // Leave unchanged if env var not found
			})

			if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
				return nil, fmt.Errorf("failed to parse config YAML: %w", err)
			}
		}
	}

	// Override with environment variables (this is the primary config method)
	if err := loadFromEnv(&config); err != nil {
		return nil, fmt.Errorf("failed to load from environment: %w", err)
	}

	// Set defaults
	if config.Defaults.Policy == "" {
		config.Defaults.Policy = types.AutoPatch
	}
	if config.Defaults.Interval == "" {
		config.Defaults.Interval = "15m"
	}
	if config.Defaults.Cooldown == "" {
		config.Defaults.Cooldown = "1h"
	}
	if len(config.Defaults.ExcludePatterns) == 0 {
		config.Defaults.ExcludePatterns = []string{"-alpha", "-beta", "-rc", "-dev", "-nightly"}
	}

	// Validate required fields
	if config.Coolify.URL == "" {
		return nil, fmt.Errorf("COOLIFY_URL is required")
	}
	if config.Coolify.Token == "" {
		return nil, fmt.Errorf("COOLIFY_TOKEN is required")
	}

	// Validate intervals and schedule
	if config.Defaults.Schedule != "" {
		// Validate cron schedule syntax
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(config.Defaults.Schedule); err != nil {
			return nil, fmt.Errorf("invalid PATROL_SCHEDULE cron syntax '%s': %w", config.Defaults.Schedule, err)
		}
	} else {
		// Validate interval if no schedule is set
		if _, err := time.ParseDuration(config.Defaults.Interval); err != nil {
			return nil, fmt.Errorf("invalid PATROL_INTERVAL: %w", err)
		}
	}
	
	if _, err := time.ParseDuration(config.Defaults.Cooldown); err != nil {
		return nil, fmt.Errorf("invalid PATROL_COOLDOWN: %w", err)
	}

	return &config, nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(config *types.Config) error {
	// Core Coolify settings (required)
	if url := os.Getenv("COOLIFY_URL"); url != "" {
		config.Coolify.URL = url
	}
	if token := os.Getenv("COOLIFY_TOKEN"); token != "" {
		config.Coolify.Token = token
	}

	// Patrol settings (optional, will use defaults if not set)
	if schedule := os.Getenv("PATROL_SCHEDULE"); schedule != "" {
		config.Defaults.Schedule = schedule
	}
	if interval := os.Getenv("PATROL_INTERVAL"); interval != "" {
		config.Defaults.Interval = interval
	}
	if policy := os.Getenv("PATROL_POLICY"); policy != "" {
		config.Defaults.Policy = types.UpdatePolicy(policy)
	}
	if cooldown := os.Getenv("PATROL_COOLDOWN"); cooldown != "" {
		config.Defaults.Cooldown = cooldown
	}

	// Exclude patterns (comma-separated)
	if patterns := os.Getenv("PATROL_EXCLUDE_PATTERNS"); patterns != "" {
		config.Defaults.ExcludePatterns = strings.Split(patterns, ",")
		// Trim whitespace from each pattern
		for i, pattern := range config.Defaults.ExcludePatterns {
			config.Defaults.ExcludePatterns[i] = strings.TrimSpace(pattern)
		}
	}

	// Apps configuration - compact format or auto-discovery
	if appsStr := os.Getenv("PATROL_APPS"); appsStr != "" {
		apps, err := parseCompactApps(appsStr)
		if err != nil {
			return fmt.Errorf("invalid PATROL_APPS format: %w", err)
		}
		config.Apps = apps
	} else if os.Getenv("PATROL_AUTO_DISCOVER") == "true" {
		// Auto-discovery mode - leave apps empty, watcher will discover them
		config.Apps = nil
	}

	return nil
}

// parseCompactApps parses the compact format: "name:uuid:image[:policy[:pin]]" semicolon-separated
// Example: "n8n:abc-123:n8nio/n8n;postgres:def-456:postgres:auto-patch:17"
func parseCompactApps(appsStr string) ([]types.AppConfig, error) {
	var apps []types.AppConfig
	
	appSpecs := strings.Split(appsStr, ";")
	for _, spec := range appSpecs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}

		parts := strings.Split(spec, ":")
		if len(parts) < 3 {
			return nil, fmt.Errorf("app spec '%s' must have at least name:uuid:image", spec)
		}

		app := types.AppConfig{
			Name:  strings.TrimSpace(parts[0]),
			UUID:  strings.TrimSpace(parts[1]),
			Image: strings.TrimSpace(parts[2]),
		}

		// Validate required fields
		if app.Name == "" {
			return nil, fmt.Errorf("app name cannot be empty in spec '%s'", spec)
		}
		if app.UUID == "" {
			return nil, fmt.Errorf("app uuid cannot be empty in spec '%s'", spec)
		}
		if app.Image == "" {
			return nil, fmt.Errorf("app image cannot be empty in spec '%s'", spec)
		}

		// Optional policy (4th field)
		if len(parts) > 3 && parts[3] != "" {
			policy := strings.TrimSpace(parts[3])
			switch policy {
			case "auto-patch", "auto-minor", "auto-all", "notify-only":
				app.Policy = types.UpdatePolicy(policy)
			default:
				return nil, fmt.Errorf("invalid policy '%s' in spec '%s'. Must be: auto-patch, auto-minor, auto-all, or notify-only", policy, spec)
			}
		}

		// Optional pin (5th field)
		if len(parts) > 4 && parts[4] != "" {
			pin := strings.TrimSpace(parts[4])
			// Validate pin is numeric
			if _, err := strconv.Atoi(pin); err != nil {
				return nil, fmt.Errorf("invalid pin '%s' in spec '%s'. Must be a number (e.g., 17)", pin, spec)
			}
			app.Pin = pin
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// LoadFromEnvOnly loads configuration entirely from environment variables (no YAML file)
func LoadFromEnvOnly() (*types.Config, error) {
	return Load("")
}

// GetUpdatePolicy returns the effective update policy for an app
func GetUpdatePolicy(app *types.AppConfig, defaults *types.DefaultsConfig) types.UpdatePolicy {
	if app.Policy != "" {
		return app.Policy
	}
	return defaults.Policy
}

// ParseInterval parses a duration string into time.Duration
func ParseInterval(interval string) (time.Duration, error) {
	return time.ParseDuration(interval)
}