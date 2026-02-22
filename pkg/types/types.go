package types

import "time"

// UpdatePolicy defines how aggressive updates should be
type UpdatePolicy string

const (
	AutoPatch UpdatePolicy = "auto-patch"
	AutoMinor UpdatePolicy = "auto-minor"
	AutoAll   UpdatePolicy = "auto-all"
	NotifyOnly UpdatePolicy = "notify-only"
)

// Config represents the main configuration file
type Config struct {
	Coolify  CoolifyConfig `yaml:"coolify"`
	Defaults DefaultsConfig `yaml:"defaults"`
	Apps     []AppConfig   `yaml:"apps,omitempty"`
}

// CoolifyConfig holds Coolify API connection details
type CoolifyConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

// DefaultsConfig holds default values for all apps
type DefaultsConfig struct {
	Policy          UpdatePolicy `yaml:"policy"`
	Interval        string       `yaml:"interval"`
	Schedule        string       `yaml:"schedule"`        // Cron schedule (takes priority over Interval)
	Cooldown        string       `yaml:"cooldown"`
	ExcludePatterns []string     `yaml:"exclude_patterns"`
}

// AppConfig defines a single application to monitor
type AppConfig struct {
	Name   string       `yaml:"name"`
	UUID   string       `yaml:"uuid"`
	Image  string       `yaml:"image"`
	Policy UpdatePolicy `yaml:"policy,omitempty"`
	Pin    string       `yaml:"pin,omitempty"`
}

// AppStatus represents current status of an app
type AppStatus struct {
	Name         string     `json:"name"`
	UUID         string     `json:"uuid"`
	Image        string     `json:"image"`
	CurrentTag   string     `json:"current_tag"`
	LatestTag    string     `json:"latest_tag"`
	Policy       string     `json:"policy"`
	UpdateNeeded bool       `json:"update_needed"`
	LastCheck    time.Time  `json:"last_check"`
	LastUpdate   *time.Time `json:"last_update,omitempty"`
	NextCheck    time.Time  `json:"next_check"`
}

// RegistryTag represents a tag from a Docker registry
type RegistryTag struct {
	Name   string
	Digest string
}

// CoolifyApplication represents an app from Coolify API
type CoolifyApplication struct {
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	DockerImage  string `json:"docker_image"`
	Status       string `json:"status"`
}

// StatusResponse is returned by /status endpoint
type StatusResponse struct {
	Status    string      `json:"status"`
	LastCheck time.Time   `json:"last_check"`
	Apps      []AppStatus `json:"apps"`
}

// HealthResponse is returned by /health endpoint
type HealthResponse struct {
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
}