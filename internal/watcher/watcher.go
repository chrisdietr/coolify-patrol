package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/chrisdietr/coolify-patrol/internal/config"
	"github.com/chrisdietr/coolify-patrol/internal/coolify"
	"github.com/chrisdietr/coolify-patrol/internal/registry"
	"github.com/chrisdietr/coolify-patrol/internal/semver"
	"github.com/chrisdietr/coolify-patrol/pkg/types"
)

// Watcher monitors applications and handles updates
type Watcher struct {
	config         *types.Config
	coolifyClient  *coolify.Client
	registryClient *registry.Client
	logger         *slog.Logger
	dryRun         bool
	
	// State tracking
	appStatuses    map[string]*types.AppStatus
	lastUpdates    map[string]time.Time
	lastCheck      time.Time
}

// NewWatcher creates a new watcher instance
func NewWatcher(cfg *types.Config, coolifyClient *coolify.Client, registryClient *registry.Client, logger *slog.Logger, dryRun bool) *Watcher {
	return &Watcher{
		config:         cfg,
		coolifyClient:  coolifyClient,
		registryClient: registryClient,
		logger:         logger,
		dryRun:         dryRun,
		appStatuses:    make(map[string]*types.AppStatus),
		lastUpdates:    make(map[string]time.Time),
	}
}

// Start begins the watcher loop
func (w *Watcher) Start(ctx context.Context, runOnce bool) error {
	scheduleInfo := w.config.Defaults.Interval
	if w.config.Defaults.Schedule != "" {
		scheduleInfo = fmt.Sprintf("cron: %s", w.config.Defaults.Schedule)
	}
	
	w.logger.Info("Starting coolify-patrol watcher",
		"dry_run", w.dryRun,
		"schedule", scheduleInfo,
		"run_once", runOnce,
	)

	// Initial check
	if err := w.checkApplications(ctx); err != nil {
		w.logger.Error("Initial check failed", "error", err)
		if runOnce {
			return err
		}
	}

	if runOnce {
		return nil
	}

	// Use cron scheduler if schedule is configured, otherwise use interval ticker
	if w.config.Defaults.Schedule != "" {
		return w.startWithCron(ctx)
	} else {
		return w.startWithInterval(ctx)
	}
}

// startWithCron starts the watcher using cron scheduling
func (w *Watcher) startWithCron(ctx context.Context) error {
	c := cron.New(cron.WithParser(cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)))
	
	_, err := c.AddFunc(w.config.Defaults.Schedule, func() {
		if err := w.checkApplications(ctx); err != nil {
			w.logger.Error("Check cycle failed", "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	w.logger.Info("Starting cron scheduler", "schedule", w.config.Defaults.Schedule)
	c.Start()
	defer c.Stop()

	// Wait for context cancellation
	<-ctx.Done()
	w.logger.Info("Watcher stopped")
	return ctx.Err()
}

// startWithInterval starts the watcher using interval-based ticker
func (w *Watcher) startWithInterval(ctx context.Context) error {
	// Parse interval
	interval, err := time.ParseDuration(w.config.Defaults.Interval)
	if err != nil {
		return fmt.Errorf("invalid interval: %w", err)
	}

	w.logger.Info("Starting interval scheduler", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Watcher stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := w.checkApplications(ctx); err != nil {
				w.logger.Error("Check cycle failed", "error", err)
			}
		}
	}
}

// checkApplications performs one complete check cycle
func (w *Watcher) checkApplications(ctx context.Context) error {
	w.lastCheck = time.Now()
	w.logger.Info("Starting check cycle")

	apps, err := w.getApplicationsToCheck(ctx)
	if err != nil {
		return fmt.Errorf("getting applications: %w", err)
	}

	w.logger.Info("Found applications to check", "count", len(apps))

	updateDelay := 30 * time.Second
	for i, app := range apps {
		if err := w.checkAndUpdateApp(ctx, app); err != nil {
			w.logger.Error("Failed to check application",
				"app", app.Name,
				"uuid", app.UUID,
				"error", err,
			)
			continue
		}

		// Add delay between updates to avoid overwhelming Coolify
		if i < len(apps)-1 {
			time.Sleep(updateDelay)
		}
	}

	w.logger.Info("Check cycle completed", "duration", time.Since(w.lastCheck))
	return nil
}

// getApplicationsToCheck returns the list of applications to check
func (w *Watcher) getApplicationsToCheck(ctx context.Context) ([]types.AppConfig, error) {
	if len(w.config.Apps) > 0 {
		// Use configured apps
		return w.config.Apps, nil
	}

	// Auto-discovery mode
	w.logger.Info("No configured apps, using auto-discovery")
	
	coolifyApps, err := w.coolifyClient.ListApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing Coolify applications: %w", err)
	}

	var apps []types.AppConfig
	for _, coolifyApp := range coolifyApps {
		// Skip apps with 'latest' tag
		_, tag := coolify.ExtractImageAndTag(coolifyApp.DockerImage)
		if tag == "latest" {
			w.logger.Warn("Skipping app with 'latest' tag",
				"app", coolifyApp.Name,
				"image", coolifyApp.DockerImage,
			)
			continue
		}

		image, _ := coolify.ExtractImageAndTag(coolifyApp.DockerImage)
		apps = append(apps, types.AppConfig{
			Name:  coolifyApp.Name,
			UUID:  coolifyApp.UUID,
			Image: image,
			// Policy will be inherited from defaults
		})
	}

	w.logger.Info("Auto-discovered applications", "count", len(apps))
	return apps, nil
}

// checkAndUpdateApp checks a single application for updates
func (w *Watcher) checkAndUpdateApp(ctx context.Context, app types.AppConfig) error {
	logger := w.logger.With("app", app.Name, "uuid", app.UUID)

	// Get current application state from Coolify
	currentApp, err := w.coolifyClient.GetApplication(ctx, app.UUID)
	if err != nil {
		return fmt.Errorf("getting current application state: %w", err)
	}

	// Extract current tag
	_, currentTag := coolify.ExtractImageAndTag(currentApp.DockerImage)
	
	logger = logger.With("current_tag", currentTag, "image", app.Image)
	
	// Check cooldown
	if lastUpdate, exists := w.lastUpdates[app.UUID]; exists {
		cooldownDuration, _ := time.ParseDuration(w.config.Defaults.Cooldown)
		if time.Since(lastUpdate) < cooldownDuration {
			logger.Debug("App in cooldown period, skipping", "last_update", lastUpdate)
			return nil
		}
	}

	// Get latest tag from registry
	latestTag, err := w.registryClient.GetLatestTag(ctx, app.Image, w.config.Defaults.ExcludePatterns)
	if err != nil {
		return fmt.Errorf("getting latest tag for %s: %w", app.Image, err)
	}

	logger = logger.With("latest_tag", latestTag)

	// Update status tracking
	status := &types.AppStatus{
		Name:       app.Name,
		UUID:       app.UUID,
		Image:      app.Image,
		CurrentTag: currentTag,
		LatestTag:  latestTag,
		Policy:     string(config.GetUpdatePolicy(&app, &w.config.Defaults)),
		LastCheck:  time.Now(),
	}

	// Check if update is needed and allowed
	policy := config.GetUpdatePolicy(&app, &w.config.Defaults)
	updateAllowed, reason := semver.IsUpdateAllowed(currentTag, latestTag, policy, app.Pin)
	status.UpdateNeeded = updateAllowed

	logger.Info("Version check completed",
		"update_needed", updateAllowed,
		"policy", policy,
		"reason", reason,
	)

	if !updateAllowed {
		logger.Info("Update not allowed or not needed", "reason", reason)
		w.appStatuses[app.UUID] = status
		return nil
	}

	// Perform update
	if w.dryRun {
		logger.Info("DRY RUN: Would update application",
			"from_tag", currentTag,
			"to_tag", latestTag,
		)
	} else {
		if err := w.performUpdate(ctx, app, latestTag, logger); err != nil {
			return fmt.Errorf("performing update: %w", err)
		}
		
		// Record successful update
		w.lastUpdates[app.UUID] = time.Now()
		updateTime := time.Now()
		status.LastUpdate = &updateTime
	}

	w.appStatuses[app.UUID] = status
	return nil
}

// performUpdate actually updates an application
func (w *Watcher) performUpdate(ctx context.Context, app types.AppConfig, newTag string, logger *slog.Logger) error {
	newImage := coolify.BuildImageReference(app.Image, newTag)
	
	logger.Info("Updating application", "new_image", newImage)

	// Update the application
	if err := w.coolifyClient.UpdateApplication(ctx, app.UUID, newImage); err != nil {
		return fmt.Errorf("updating application config: %w", err)
	}

	// Trigger restart/redeploy
	if err := w.coolifyClient.RestartApplication(ctx, app.UUID); err != nil {
		return fmt.Errorf("restarting application: %w", err)
	}

	logger.Info("Application updated successfully",
		"new_image", newImage,
		"restart_triggered", true,
	)

	return nil
}

// GetStatus returns current status of all watched applications
func (w *Watcher) GetStatus() *types.StatusResponse {
	var apps []types.AppStatus
	for _, status := range w.appStatuses {
		apps = append(apps, *status)
	}

	return &types.StatusResponse{
		Status:    "running",
		LastCheck: w.lastCheck,
		Apps:      apps,
	}
}

// DiscoverApps returns a list of all Coolify applications for discovery
func (w *Watcher) DiscoverApps(ctx context.Context) ([]types.CoolifyApplication, error) {
	return w.coolifyClient.ListApplications(ctx)
}

// GenerateSampleConfig generates a sample configuration based on discovered apps
func (w *Watcher) GenerateSampleConfig(ctx context.Context) (*types.Config, error) {
	apps, err := w.DiscoverApps(ctx)
	if err != nil {
		return nil, err
	}

	config := &types.Config{
		Coolify: w.config.Coolify,
		Defaults: types.DefaultsConfig{
			Policy:          types.AutoPatch,
			Interval:        "15m",
			Cooldown:        "1h",
			ExcludePatterns: []string{"-alpha", "-beta", "-rc", "-dev", "-nightly"},
		},
	}

	for _, app := range apps {
		// Skip latest tags
		_, tag := coolify.ExtractImageAndTag(app.DockerImage)
		if tag == "latest" {
			continue
		}

		image, _ := coolify.ExtractImageAndTag(app.DockerImage)
		appConfig := types.AppConfig{
			Name:  app.Name,
			UUID:  app.UUID,
			Image: image,
		}

		// Add suggested pin for well-known images
		if strings.Contains(image, "postgres") {
			if ver, err := semver.ParseVersion(tag); err == nil {
				appConfig.Pin = fmt.Sprintf("%d", ver.Major)
			}
		}

		config.Apps = append(config.Apps, appConfig)
	}

	return config, nil
}