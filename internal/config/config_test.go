package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chrisdietr/coolify-patrol/pkg/types"
)

func TestLoad(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "patrol.yaml")
	
	configContent := `
coolify:
  url: http://localhost:8000
  token: ${COOLIFY_API_TOKEN}

defaults:
  policy: auto-patch
  interval: 10m
  cooldown: 30m
  exclude_patterns:
    - "-alpha"
    - "-beta"

apps:
  - name: test-app
    uuid: test-uuid
    image: nginx
    policy: auto-minor
    pin: "1"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Set environment variable
	os.Setenv("COOLIFY_API_TOKEN", "test-token")
	defer os.Unsetenv("COOLIFY_API_TOKEN")

	// Load config
	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify configuration
	if cfg.Coolify.URL != "http://localhost:8000" {
		t.Errorf("expected URL 'http://localhost:8000', got '%s'", cfg.Coolify.URL)
	}

	if cfg.Coolify.Token != "test-token" {
		t.Errorf("expected token 'test-token', got '%s'", cfg.Coolify.Token)
	}

	if cfg.Defaults.Policy != types.AutoPatch {
		t.Errorf("expected policy 'auto-patch', got '%s'", cfg.Defaults.Policy)
	}

	if cfg.Defaults.Interval != "10m" {
		t.Errorf("expected interval '10m', got '%s'", cfg.Defaults.Interval)
	}

	if len(cfg.Apps) != 1 {
		t.Errorf("expected 1 app, got %d", len(cfg.Apps))
		return
	}

	app := cfg.Apps[0]
	if app.Name != "test-app" {
		t.Errorf("expected app name 'test-app', got '%s'", app.Name)
	}
	if app.Policy != types.AutoMinor {
		t.Errorf("expected app policy 'auto-minor', got '%s'", app.Policy)
	}
	if app.Pin != "1" {
		t.Errorf("expected app pin '1', got '%s'", app.Pin)
	}
}

func TestLoadDefaults(t *testing.T) {
	// Create minimal config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "patrol.yaml")
	
	configContent := `
coolify:
  url: http://localhost:8000
  token: test-token
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load config
	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify defaults
	if cfg.Defaults.Policy != types.AutoPatch {
		t.Errorf("expected default policy 'auto-patch', got '%s'", cfg.Defaults.Policy)
	}

	if cfg.Defaults.Interval != "15m" {
		t.Errorf("expected default interval '15m', got '%s'", cfg.Defaults.Interval)
	}

	if cfg.Defaults.Cooldown != "1h" {
		t.Errorf("expected default cooldown '1h', got '%s'", cfg.Defaults.Cooldown)
	}

	expectedPatterns := []string{"-alpha", "-beta", "-rc", "-dev", "-nightly"}
	if len(cfg.Defaults.ExcludePatterns) != len(expectedPatterns) {
		t.Errorf("expected %d exclude patterns, got %d", len(expectedPatterns), len(cfg.Defaults.ExcludePatterns))
		return
	}

	for i, pattern := range expectedPatterns {
		if cfg.Defaults.ExcludePatterns[i] != pattern {
			t.Errorf("expected pattern '%s' at index %d, got '%s'", pattern, i, cfg.Defaults.ExcludePatterns[i])
		}
	}
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name: "missing coolify url",
			config: `
coolify:
  token: test-token
`,
			expectError: true,
		},
		{
			name: "missing coolify token",
			config: `
coolify:
  url: http://localhost:8000
`,
			expectError: true,
		},
		{
			name: "invalid interval",
			config: `
coolify:
  url: http://localhost:8000
  token: test-token
defaults:
  interval: invalid
`,
			expectError: true,
		},
		{
			name: "invalid cooldown",
			config: `
coolify:
  url: http://localhost:8000
  token: test-token
defaults:
  cooldown: invalid
`,
			expectError: true,
		},
		{
			name: "valid minimal config",
			config: `
coolify:
  url: http://localhost:8000
  token: test-token
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "patrol.yaml")
			
			if err := os.WriteFile(configFile, []byte(tt.config), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			_, err := Load(configFile)
			
			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetUpdatePolicy(t *testing.T) {
	defaults := &types.DefaultsConfig{
		Policy: types.AutoPatch,
	}

	tests := []struct {
		name     string
		app      *types.AppConfig
		expected types.UpdatePolicy
	}{
		{
			name: "app with explicit policy",
			app: &types.AppConfig{
				Policy: types.AutoMinor,
			},
			expected: types.AutoMinor,
		},
		{
			name: "app without policy uses default",
			app: &types.AppConfig{
				Policy: "",
			},
			expected: types.AutoPatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUpdatePolicy(tt.app, defaults)
			if result != tt.expected {
				t.Errorf("expected policy '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestParseInterval(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"10m", 10 * time.Minute, false},
		{"1h", time.Hour, false},
		{"30s", 30 * time.Second, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseInterval(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input '%s', got nil", tt.input)
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error for input '%s': %v", tt.input, err)
				return
			}
			
			if result != tt.expected {
				t.Errorf("expected duration %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEnvVarSubstitution(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_URL", "http://test.com")
	os.Setenv("TEST_TOKEN", "secret-token")
	defer func() {
		os.Unsetenv("TEST_URL")
		os.Unsetenv("TEST_TOKEN")
	}()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "patrol.yaml")
	
	configContent := `
coolify:
  url: ${TEST_URL}
  token: ${TEST_TOKEN}

defaults:
  policy: auto-patch
  interval: 15m
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Coolify.URL != "http://test.com" {
		t.Errorf("expected URL 'http://test.com', got '%s'", cfg.Coolify.URL)
	}

	if cfg.Coolify.Token != "secret-token" {
		t.Errorf("expected token 'secret-token', got '%s'", cfg.Coolify.Token)
	}
}