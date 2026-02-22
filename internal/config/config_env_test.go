package config

import (
	"os"
	"testing"

	"github.com/chrisdietr/coolify-patrol/pkg/types"
)

func TestLoadFromEnvOnly(t *testing.T) {
	// Clean environment
	cleanEnv := func() {
		os.Unsetenv("COOLIFY_URL")
		os.Unsetenv("COOLIFY_TOKEN")
		os.Unsetenv("PATROL_INTERVAL")
		os.Unsetenv("PATROL_POLICY")
		os.Unsetenv("PATROL_COOLDOWN")
		os.Unsetenv("PATROL_APPS")
		os.Unsetenv("PATROL_AUTO_DISCOVER")
		os.Unsetenv("PATROL_EXCLUDE_PATTERNS")
	}
	
	defer cleanEnv()
	cleanEnv() // Clean before test

	// Set required env vars
	os.Setenv("COOLIFY_URL", "http://localhost:8000")
	os.Setenv("COOLIFY_TOKEN", "test-token")
	os.Setenv("PATROL_AUTO_DISCOVER", "true")

	cfg, err := LoadFromEnvOnly()
	if err != nil {
		t.Fatalf("failed to load from env: %v", err)
	}

	if cfg.Coolify.URL != "http://localhost:8000" {
		t.Errorf("expected URL 'http://localhost:8000', got '%s'", cfg.Coolify.URL)
	}

	if cfg.Coolify.Token != "test-token" {
		t.Errorf("expected token 'test-token', got '%s'", cfg.Coolify.Token)
	}

	// Should have defaults
	if cfg.Defaults.Policy != types.AutoPatch {
		t.Errorf("expected default policy 'auto-patch', got '%s'", cfg.Defaults.Policy)
	}

	if cfg.Defaults.Interval != "15m" {
		t.Errorf("expected default interval '15m', got '%s'", cfg.Defaults.Interval)
	}
}

func TestLoadFromEnvWithCustomSettings(t *testing.T) {
	// Clean environment
	cleanEnv := func() {
		os.Unsetenv("COOLIFY_URL")
		os.Unsetenv("COOLIFY_TOKEN")
		os.Unsetenv("PATROL_INTERVAL")
		os.Unsetenv("PATROL_POLICY")
		os.Unsetenv("PATROL_COOLDOWN")
		os.Unsetenv("PATROL_APPS")
		os.Unsetenv("PATROL_AUTO_DISCOVER")
		os.Unsetenv("PATROL_EXCLUDE_PATTERNS")
	}
	
	defer cleanEnv()
	cleanEnv() // Clean before test

	// Set environment variables
	os.Setenv("COOLIFY_URL", "https://coolify.example.com")
	os.Setenv("COOLIFY_TOKEN", "custom-token")
	os.Setenv("PATROL_INTERVAL", "10m")
	os.Setenv("PATROL_POLICY", "auto-minor")
	os.Setenv("PATROL_COOLDOWN", "30m")
	os.Setenv("PATROL_EXCLUDE_PATTERNS", "-alpha,-beta,-rc")

	cfg, err := LoadFromEnvOnly()
	if err != nil {
		t.Fatalf("failed to load from env: %v", err)
	}

	if cfg.Coolify.URL != "https://coolify.example.com" {
		t.Errorf("expected URL 'https://coolify.example.com', got '%s'", cfg.Coolify.URL)
	}

	if cfg.Defaults.Policy != types.AutoMinor {
		t.Errorf("expected policy 'auto-minor', got '%s'", cfg.Defaults.Policy)
	}

	if cfg.Defaults.Interval != "10m" {
		t.Errorf("expected interval '10m', got '%s'", cfg.Defaults.Interval)
	}

	if cfg.Defaults.Cooldown != "30m" {
		t.Errorf("expected cooldown '30m', got '%s'", cfg.Defaults.Cooldown)
	}

	expectedPatterns := []string{"-alpha", "-beta", "-rc"}
	if len(cfg.Defaults.ExcludePatterns) != len(expectedPatterns) {
		t.Errorf("expected %d exclude patterns, got %d", len(expectedPatterns), len(cfg.Defaults.ExcludePatterns))
	}

	for i, pattern := range expectedPatterns {
		if cfg.Defaults.ExcludePatterns[i] != pattern {
			t.Errorf("expected pattern '%s' at index %d, got '%s'", pattern, i, cfg.Defaults.ExcludePatterns[i])
		}
	}
}

func TestParseCompactApps(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []types.AppConfig
		expectErr bool
	}{
		{
			name:  "single app basic",
			input: "n8n:abc123:n8nio/n8n",
			expected: []types.AppConfig{
				{
					Name:  "n8n",
					UUID:  "abc123",
					Image: "n8nio/n8n",
				},
			},
		},
		{
			name:  "single app with policy",
			input: "postgres:def456:postgres:auto-patch",
			expected: []types.AppConfig{
				{
					Name:   "postgres",
					UUID:   "def456",
					Image:  "postgres",
					Policy: types.AutoPatch,
				},
			},
		},
		{
			name:  "single app with policy and pin",
			input: "postgres:def456:postgres:auto-patch:17",
			expected: []types.AppConfig{
				{
					Name:   "postgres",
					UUID:   "def456",
					Image:  "postgres",
					Policy: types.AutoPatch,
					Pin:    "17",
				},
			},
		},
		{
			name:  "multiple apps",
			input: "n8n:abc123:n8nio/n8n;postgres:def456:postgres:auto-patch:17;redis:ghi789:redis:auto-minor",
			expected: []types.AppConfig{
				{
					Name:  "n8n",
					UUID:  "abc123",
					Image: "n8nio/n8n",
				},
				{
					Name:   "postgres",
					UUID:   "def456",
					Image:  "postgres",
					Policy: types.AutoPatch,
					Pin:    "17",
				},
				{
					Name:   "redis",
					UUID:   "ghi789",
					Image:  "redis",
					Policy: types.AutoMinor,
				},
			},
		},
		{
			name:      "missing required fields",
			input:     "n8n:abc123",
			expectErr: true,
		},
		{
			name:      "empty name",
			input:     ":abc123:n8nio/n8n",
			expectErr: true,
		},
		{
			name:      "empty uuid",
			input:     "n8n::n8nio/n8n",
			expectErr: true,
		},
		{
			name:      "empty image",
			input:     "n8n:abc123:",
			expectErr: true,
		},
		{
			name:      "invalid policy",
			input:     "n8n:abc123:n8nio/n8n:invalid-policy",
			expectErr: true,
		},
		{
			name:      "invalid pin",
			input:     "postgres:def456:postgres:auto-patch:not-a-number",
			expectErr: true,
		},
		{
			name:     "empty spec ignored",
			input:    "n8n:abc123:n8nio/n8n;;postgres:def456:postgres",
			expected: []types.AppConfig{
				{
					Name:  "n8n",
					UUID:  "abc123",
					Image: "n8nio/n8n",
				},
				{
					Name:  "postgres",
					UUID:  "def456",
					Image: "postgres",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCompactApps(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error for input '%s', got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d apps, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i].Name != expected.Name {
					t.Errorf("app %d: expected name '%s', got '%s'", i, expected.Name, result[i].Name)
				}
				if result[i].UUID != expected.UUID {
					t.Errorf("app %d: expected uuid '%s', got '%s'", i, expected.UUID, result[i].UUID)
				}
				if result[i].Image != expected.Image {
					t.Errorf("app %d: expected image '%s', got '%s'", i, expected.Image, result[i].Image)
				}
				if result[i].Policy != expected.Policy {
					t.Errorf("app %d: expected policy '%s', got '%s'", i, expected.Policy, result[i].Policy)
				}
				if result[i].Pin != expected.Pin {
					t.Errorf("app %d: expected pin '%s', got '%s'", i, expected.Pin, result[i].Pin)
				}
			}
		})
	}
}

func TestLoadFromEnvWithApps(t *testing.T) {
	// Clean environment
	cleanEnv := func() {
		os.Unsetenv("COOLIFY_URL")
		os.Unsetenv("COOLIFY_TOKEN")
		os.Unsetenv("PATROL_APPS")
		os.Unsetenv("PATROL_AUTO_DISCOVER")
	}
	
	defer cleanEnv()
	cleanEnv() // Clean before test

	// Set environment variables
	os.Setenv("COOLIFY_URL", "http://localhost:8000")
	os.Setenv("COOLIFY_TOKEN", "test-token")
	os.Setenv("PATROL_APPS", "n8n:abc123:n8nio/n8n:auto-minor;postgres:def456:postgres:auto-patch:17")

	cfg, err := LoadFromEnvOnly()
	if err != nil {
		t.Fatalf("failed to load from env: %v", err)
	}

	if len(cfg.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(cfg.Apps))
	}

	// Check first app
	app1 := cfg.Apps[0]
	if app1.Name != "n8n" {
		t.Errorf("expected app1 name 'n8n', got '%s'", app1.Name)
	}
	if app1.Policy != types.AutoMinor {
		t.Errorf("expected app1 policy 'auto-minor', got '%s'", app1.Policy)
	}

	// Check second app
	app2 := cfg.Apps[1]
	if app2.Name != "postgres" {
		t.Errorf("expected app2 name 'postgres', got '%s'", app2.Name)
	}
	if app2.Policy != types.AutoPatch {
		t.Errorf("expected app2 policy 'auto-patch', got '%s'", app2.Policy)
	}
	if app2.Pin != "17" {
		t.Errorf("expected app2 pin '17', got '%s'", app2.Pin)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	// Clean environment
	cleanEnv := func() {
		os.Unsetenv("COOLIFY_URL")
		os.Unsetenv("COOLIFY_TOKEN")
	}
	
	defer cleanEnv()
	cleanEnv() // Clean before test

	tests := []struct {
		name      string
		setURL    bool
		setToken  bool
		expectErr string
	}{
		{
			name:      "missing both",
			setURL:    false,
			setToken:  false,
			expectErr: "COOLIFY_URL is required",
		},
		{
			name:      "missing token",
			setURL:    true,
			setToken:  false,
			expectErr: "COOLIFY_TOKEN is required",
		},
		{
			name:      "has both",
			setURL:    true,
			setToken:  true,
			expectErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnv()
			
			if tt.setURL {
				os.Setenv("COOLIFY_URL", "http://localhost:8000")
			}
			if tt.setToken {
				os.Setenv("COOLIFY_TOKEN", "test-token")
			}

			_, err := LoadFromEnvOnly()

			if tt.expectErr != "" {
				if err == nil {
					t.Errorf("expected error '%s', got nil", tt.expectErr)
					return
				}
				if err.Error() != tt.expectErr {
					t.Errorf("expected error '%s', got '%s'", tt.expectErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}