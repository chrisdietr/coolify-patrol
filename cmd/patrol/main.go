package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"

	"github.com/chrisdietr/coolify-patrol/internal/config"
	"github.com/chrisdietr/coolify-patrol/internal/coolify"
	"github.com/chrisdietr/coolify-patrol/internal/registry"
	"github.com/chrisdietr/coolify-patrol/internal/server"
	"github.com/chrisdietr/coolify-patrol/internal/watcher"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	var (
		configPath  = flag.String("config", "/config/patrol.yaml", "Path to patrol.yaml")
		dryRun      = flag.Bool("dry-run", false, "Log what would be updated without making changes")
		once        = flag.Bool("once", false, "Run one check cycle and exit")
		interval    = flag.String("interval", "", "Override check interval (e.g., 5m, 1h)")
		schedule    = flag.String("schedule", "", "Override cron schedule (e.g., '*/15 * * * *', '0 3 * * *')")
		logFormat   = flag.String("log-format", "json", "Log format: json or text")
		port        = flag.Int("port", 8080, "HTTP server port")
		showVersion = flag.Bool("version", false, "Print version and exit")
		command     = flag.String("command", "", "Command to run: check, status, discover")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("coolify-patrol %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Built: %s\n", date)
		return
	}

	// Show help with env vars if requested
	if *command == "help" || (len(os.Args) > 1 && (os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-help")) {
		showHelp()
		return
	}

	// Setup logger
	var logger *slog.Logger
	if *logFormat == "text" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	// Handle dry-run from environment
	if os.Getenv("PATROL_DRY_RUN") == "true" {
		*dryRun = true
	}

	// Handle port from environment
	if portEnv := os.Getenv("PATROL_PORT"); portEnv != "" {
		if p, err := strconv.Atoi(portEnv); err == nil {
			*port = p
		}
	}

	// Handle config path - allow running without YAML if env vars are present
	var configExists bool
	if *configPath == "/config/patrol.yaml" {
		if _, err := os.Stat(*configPath); os.IsNotExist(err) {
			// Try current directory
			if _, err := os.Stat("patrol.yaml"); err == nil {
				*configPath = "patrol.yaml"
				configExists = true
			}
		} else {
			configExists = true
		}
	} else {
		// Custom path specified
		if _, err := os.Stat(*configPath); err == nil {
			configExists = true
		}
	}

	// If no config file exists, check if we have required env vars for env-only mode
	if !configExists {
		if os.Getenv("COOLIFY_URL") == "" || os.Getenv("COOLIFY_TOKEN") == "" {
			logger.Error("No config file found and required environment variables missing",
				"config_path", *configPath,
				"required_env_vars", "COOLIFY_URL, COOLIFY_TOKEN",
			)
			fmt.Fprintf(os.Stderr, "Error: No configuration found.\n\n")
			fmt.Fprintf(os.Stderr, "Either provide a config file or set environment variables:\n")
			fmt.Fprintf(os.Stderr, "  Required: COOLIFY_URL, COOLIFY_TOKEN\n")
			fmt.Fprintf(os.Stderr, "  Optional: PATROL_APPS or PATROL_AUTO_DISCOVER=true\n")
			fmt.Fprintf(os.Stderr, "\nExample:\n")
			fmt.Fprintf(os.Stderr, "  export COOLIFY_URL=http://localhost:8000\n")
			fmt.Fprintf(os.Stderr, "  export COOLIFY_TOKEN=your-api-token\n")
			fmt.Fprintf(os.Stderr, "  export PATROL_AUTO_DISCOVER=true\n")
			os.Exit(1)
		}
		// Use env-only mode
		*configPath = ""
	}

	logger.Info("Starting coolify-patrol",
		"version", version,
		"config", *configPath,
		"dry_run", *dryRun,
	)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Override schedule or interval if specified (schedule takes priority)
	if *schedule != "" {
		// Validate cron schedule
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(*schedule); err != nil {
			logger.Error("Invalid cron schedule format", "schedule", *schedule, "error", err)
			os.Exit(1)
		}
		cfg.Defaults.Schedule = *schedule
		cfg.Defaults.Interval = "" // Clear interval when schedule is set
	} else if *interval != "" {
		if _, err := time.ParseDuration(*interval); err != nil {
			logger.Error("Invalid interval format", "interval", *interval, "error", err)
			os.Exit(1)
		}
		cfg.Defaults.Interval = *interval
		cfg.Defaults.Schedule = "" // Clear schedule when interval is set
	}

	// Create clients
	coolifyClient := coolify.NewClient(cfg.Coolify.URL, cfg.Coolify.Token)
	registryClient := registry.NewClient()

	// Test Coolify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := coolifyClient.TestConnection(ctx); err != nil {
		logger.Error("Failed to connect to Coolify", "error", err)
		cancel()
		os.Exit(1)
	}
	cancel()

	// Create watcher
	w := watcher.NewWatcher(cfg, coolifyClient, registryClient, logger, *dryRun)

	// Handle commands
	switch *command {
	case "check":
		*once = true
	case "status":
		status := w.GetStatus()
		if err := json.NewEncoder(os.Stdout).Encode(status); err != nil {
			logger.Error("Failed to encode status", "error", err)
			os.Exit(1)
		}
		return
	case "discover":
		handleDiscoverCommand(w, logger)
		return
	}

	// Setup context for graceful shutdown
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server (unless running once)
	var httpServer *server.Server
	if !*once {
		httpServer = server.NewServer(w, logger, *port, version)
		go func() {
			if err := httpServer.Start(); err != nil && err != http.ErrServerClosed {
				logger.Error("HTTP server failed", "error", err)
				cancel()
			}
		}()
	}

	// Start watcher in goroutine
	watcherDone := make(chan error, 1)
	go func() {
		watcherDone <- w.Start(ctx, *once)
	}()

	// Wait for completion or signal
	select {
	case sig := <-sigCh:
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
		
		// Stop HTTP server if running
		if httpServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := httpServer.Stop(shutdownCtx); err != nil {
				logger.Error("Failed to stop HTTP server", "error", err)
			}
			shutdownCancel()
		}
		
		// Wait for watcher to finish
		<-watcherDone
		
	case err := <-watcherDone:
		if err != nil && err != context.Canceled {
			logger.Error("Watcher failed", "error", err)
			os.Exit(1)
		}
		
		// Stop HTTP server if running
		if httpServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := httpServer.Stop(shutdownCtx); err != nil {
				logger.Error("Failed to stop HTTP server", "error", err)
			}
			shutdownCancel()
		}
	}

	logger.Info("Coolify patrol stopped")
}

func handleDiscoverCommand(w *watcher.Watcher, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	apps, err := w.DiscoverApps(ctx)
	if err != nil {
		logger.Error("Failed to discover applications", "error", err)
		os.Exit(1)
	}

	logger.Info("Discovered applications", "count", len(apps))

	fmt.Println("\n# Discovered Coolify Applications")
	fmt.Println("# Copy this configuration to your patrol.yaml file")

	// Generate sample config
	sampleConfig, err := w.GenerateSampleConfig(ctx)
	if err != nil {
		logger.Error("Failed to generate sample config", "error", err)
		os.Exit(1)
	}

	configYAML, err := yaml.Marshal(sampleConfig)
	if err != nil {
		logger.Error("Failed to marshal config", "error", err)
		os.Exit(1)
	}

	fmt.Print(string(configYAML))

	fmt.Println("\n# Applications found:")
	for _, app := range apps {
		fmt.Printf("# - %s (%s): %s\n", app.Name, app.UUID, app.DockerImage)
	}
}

func showHelp() {
	fmt.Printf("coolify-patrol %s - Automated Docker image updates for Coolify\n\n", version)
	
	fmt.Println("USAGE:")
	fmt.Println("  coolify-patrol [flags] [command]")
	
	fmt.Println("\nFLAGS:")
	flag.PrintDefaults()
	
	fmt.Println("\nCOMMANDS:")
	fmt.Println("  check                 Run one check cycle and exit")
	fmt.Println("  status                Print current status of all watched apps")
	fmt.Println("  discover              List all Coolify apps and suggest config")
	
	fmt.Println("\nCONFIGURATION:")
	fmt.Println("  Coolify Patrol can be configured via YAML file OR environment variables.")
	fmt.Println("  Environment variables take precedence over YAML settings.")
	
	fmt.Println("\n  Required Environment Variables:")
	fmt.Println("    COOLIFY_URL         Coolify server URL (e.g., http://localhost:8000)")
	fmt.Println("    COOLIFY_TOKEN       Coolify API token")
	
	fmt.Println("\n  Optional Environment Variables:")
	fmt.Println("    PATROL_SCHEDULE     Cron schedule (takes priority over PATROL_INTERVAL)")
	fmt.Println("    PATROL_INTERVAL     Check interval (default: 15m)")
	fmt.Println("    PATROL_POLICY       Default policy: auto-patch|auto-minor|auto-all|notify-only")
	fmt.Println("    PATROL_COOLDOWN     Cooldown between updates (default: 1h)")
	fmt.Println("    PATROL_DRY_RUN      Set to 'true' for dry-run mode")
	fmt.Println("    PATROL_PORT         HTTP server port (default: 8080)")
	fmt.Println("    PATROL_EXCLUDE_PATTERNS  Comma-separated patterns to exclude (e.g., '-alpha,-beta')")
	
	fmt.Println("\n  App Configuration (choose one):")
	fmt.Println("    PATROL_AUTO_DISCOVER=true    Auto-discover all Coolify applications")
	fmt.Println("    PATROL_APPS                  Specific apps in compact format")
	
	fmt.Println("\n  PATROL_APPS Format:")
	fmt.Println("    \"name:uuid:image[:policy[:pin]]\" separated by semicolons")
	
	fmt.Println("\n  Examples:")
	fmt.Println("    # Auto-discover all apps")
	fmt.Println("    PATROL_AUTO_DISCOVER=true")
	
	fmt.Println("\n    # Specific apps")
	fmt.Println("    PATROL_APPS=\"n8n:abc123:n8nio/n8n;postgres:def456:postgres:auto-patch:17\"")
	
	fmt.Println("\n    # With interval")
	fmt.Println("    COOLIFY_URL=http://localhost:8000")
	fmt.Println("    COOLIFY_TOKEN=your-token-here")
	fmt.Println("    PATROL_AUTO_DISCOVER=true")
	fmt.Println("    PATROL_INTERVAL=10m")
	
	fmt.Println("\n    # With cron schedule")
	fmt.Println("    COOLIFY_URL=http://localhost:8000")
	fmt.Println("    COOLIFY_TOKEN=your-token-here")
	fmt.Println("    PATROL_AUTO_DISCOVER=true")
	fmt.Println("    PATROL_SCHEDULE=\"*/15 * * * *\"  # Every 15 minutes")
	fmt.Println("    # PATROL_SCHEDULE=\"0 3 * * *\"    # Daily at 3 AM")
	
	fmt.Println("\nFor more information, see: https://github.com/chrisdietr/coolify-patrol")
}