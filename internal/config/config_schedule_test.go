package config

import (
	"os"
	"testing"
)

func TestLoadFromEnvWithSchedule(t *testing.T) {
	// Clean environment
	cleanEnv := func() {
		os.Unsetenv("COOLIFY_URL")
		os.Unsetenv("COOLIFY_TOKEN")
		os.Unsetenv("PATROL_INTERVAL")
		os.Unsetenv("PATROL_SCHEDULE")
		os.Unsetenv("PATROL_AUTO_DISCOVER")
	}
	
	defer cleanEnv()
	cleanEnv() // Clean before test

	tests := []struct {
		name      string
		schedule  string
		interval  string
		expectErr bool
		errMsg    string
	}{
		{
			name:     "valid cron schedule - every 15 minutes",
			schedule: "*/15 * * * *",
			interval: "",
		},
		{
			name:     "valid cron schedule - daily at 3am",
			schedule: "0 3 * * *",
			interval: "",
		},
		{
			name:     "valid cron schedule - every hour",
			schedule: "0 * * * *",
			interval: "",
		},
		{
			name:     "valid cron schedule - twice daily",
			schedule: "0 9,21 * * *",
			interval: "",
		},
		{
			name:     "valid cron schedule - weekdays only",
			schedule: "0 9 * * 1-5",
			interval: "",
		},
		{
			name:      "invalid cron schedule - too many fields",
			schedule:  "0 0 0 0 0 0",
			interval:  "",
			expectErr: true,
		},
		{
			name:      "invalid cron schedule - invalid minute",
			schedule:  "60 * * * *",
			interval:  "",
			expectErr: true,
		},
		{
			name:      "invalid cron schedule - invalid hour",
			schedule:  "0 25 * * *",
			interval:  "",
			expectErr: true,
		},
		{
			name:      "invalid cron schedule - invalid day",
			schedule:  "0 0 32 * *",
			interval:  "",
			expectErr: true,
		},
		{
			name:      "invalid cron schedule - invalid month",
			schedule:  "0 0 1 13 *",
			interval:  "",
			expectErr: true,
		},
		{
			name:      "invalid cron schedule - invalid weekday",
			schedule:  "0 0 * * 8",
			interval:  "",
			expectErr: true,
		},
		{
			name:     "schedule takes priority over interval",
			schedule: "*/30 * * * *",
			interval: "5m",
		},
		{
			name:      "no schedule, valid interval",
			schedule:  "",
			interval:  "10m",
			expectErr: false,
		},
		{
			name:      "no schedule, invalid interval",
			schedule:  "",
			interval:  "invalid",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnv()
			
			// Set required env vars
			os.Setenv("COOLIFY_URL", "http://localhost:8000")
			os.Setenv("COOLIFY_TOKEN", "test-token")
			os.Setenv("PATROL_AUTO_DISCOVER", "true")

			if tt.schedule != "" {
				os.Setenv("PATROL_SCHEDULE", tt.schedule)
			}
			if tt.interval != "" {
				os.Setenv("PATROL_INTERVAL", tt.interval)
			}

			cfg, err := LoadFromEnvOnly()

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error for schedule '%s', got nil", tt.schedule)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for schedule '%s': %v", tt.schedule, err)
				return
			}

			// Verify schedule is set correctly
			if tt.schedule != "" {
				if cfg.Defaults.Schedule != tt.schedule {
					t.Errorf("expected schedule '%s', got '%s'", tt.schedule, cfg.Defaults.Schedule)
				}
			} else if tt.interval != "" {
				if cfg.Defaults.Interval != tt.interval {
					t.Errorf("expected interval '%s', got '%s'", tt.interval, cfg.Defaults.Interval)
				}
			}
		})
	}
}

func TestSchedulePriorityOverInterval(t *testing.T) {
	// Clean environment
	cleanEnv := func() {
		os.Unsetenv("COOLIFY_URL")
		os.Unsetenv("COOLIFY_TOKEN")
		os.Unsetenv("PATROL_INTERVAL")
		os.Unsetenv("PATROL_SCHEDULE")
		os.Unsetenv("PATROL_AUTO_DISCOVER")
	}
	
	defer cleanEnv()
	cleanEnv() // Clean before test

	// Set environment variables with both schedule and interval
	os.Setenv("COOLIFY_URL", "http://localhost:8000")
	os.Setenv("COOLIFY_TOKEN", "test-token")
	os.Setenv("PATROL_AUTO_DISCOVER", "true")
	os.Setenv("PATROL_SCHEDULE", "*/30 * * * *")
	os.Setenv("PATROL_INTERVAL", "5m")

	cfg, err := LoadFromEnvOnly()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Schedule should be set
	if cfg.Defaults.Schedule != "*/30 * * * *" {
		t.Errorf("expected schedule '*/30 * * * *', got '%s'", cfg.Defaults.Schedule)
	}

	// Interval should also be set (both can coexist, but schedule takes priority in watcher)
	if cfg.Defaults.Interval != "5m" {
		t.Errorf("expected interval '5m', got '%s'", cfg.Defaults.Interval)
	}
}

func TestCronScheduleExamples(t *testing.T) {
	// Clean environment
	cleanEnv := func() {
		os.Unsetenv("COOLIFY_URL")
		os.Unsetenv("COOLIFY_TOKEN")
		os.Unsetenv("PATROL_SCHEDULE")
		os.Unsetenv("PATROL_AUTO_DISCOVER")
	}
	
	defer cleanEnv()

	examples := []struct {
		name     string
		schedule string
		desc     string
	}{
		{
			name:     "every 15 minutes",
			schedule: "*/15 * * * *",
			desc:     "Run every 15 minutes",
		},
		{
			name:     "hourly at minute 0",
			schedule: "0 * * * *",
			desc:     "Run at the top of every hour",
		},
		{
			name:     "daily at 3 AM",
			schedule: "0 3 * * *",
			desc:     "Run daily at 3:00 AM",
		},
		{
			name:     "twice daily",
			schedule: "0 9,21 * * *",
			desc:     "Run at 9 AM and 9 PM daily",
		},
		{
			name:     "weekdays at 9 AM",
			schedule: "0 9 * * 1-5",
			desc:     "Run at 9 AM Monday through Friday",
		},
		{
			name:     "first day of month",
			schedule: "0 0 1 * *",
			desc:     "Run at midnight on the 1st day of each month",
		},
		{
			name:     "every 6 hours",
			schedule: "0 */6 * * *",
			desc:     "Run every 6 hours",
		},
	}

	for _, example := range examples {
		t.Run(example.name, func(t *testing.T) {
			cleanEnv()
			
			os.Setenv("COOLIFY_URL", "http://localhost:8000")
			os.Setenv("COOLIFY_TOKEN", "test-token")
			os.Setenv("PATROL_AUTO_DISCOVER", "true")
			os.Setenv("PATROL_SCHEDULE", example.schedule)

			cfg, err := LoadFromEnvOnly()
			if err != nil {
				t.Errorf("failed to parse valid cron schedule '%s' (%s): %v", example.schedule, example.desc, err)
				return
			}

			if cfg.Defaults.Schedule != example.schedule {
				t.Errorf("expected schedule '%s', got '%s'", example.schedule, cfg.Defaults.Schedule)
			}
		})
	}
}