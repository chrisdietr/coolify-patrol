# Coolify Patrol 🔄

[![Go](https://img.shields.io/badge/go-1.23-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Automated Docker image update service for Coolify servers. Monitors Docker registries for new releases and automatically updates applications via the Coolify API using configurable semver-aware update policies.

## Features

- 🔍 **Registry Monitoring** - Monitors Docker Hub and GitHub Container Registry (GHCR)
- 📋 **Semver-Aware Updates** - Intelligent version comparison and update policies
- 🎯 **Update Policies** - Fine-grained control: auto-patch, auto-minor, auto-all, notify-only
- 📌 **Major Version Pinning** - Pin applications to specific major versions
- 🔄 **Auto-Discovery** - Automatically discover and monitor all Coolify applications
- ⏱️ **Rate Limiting** - Handles Docker Hub rate limits gracefully
- 🏥 **Health Monitoring** - Built-in health and status endpoints
- 🧪 **Dry Run Mode** - Test updates without making changes
- 📊 **Structured Logging** - JSON logs for easy parsing and monitoring

## Quick Start

### 1. Environment Variables (Recommended)

The easiest way to deploy in Coolify is using environment variables:

```bash
# Required
export COOLIFY_URL="http://localhost:8000"
export COOLIFY_TOKEN="your-api-token-here"

# Auto-discover all applications
export PATROL_AUTO_DISCOVER=true

# Run
docker run --rm \
  -e COOLIFY_URL \
  -e COOLIFY_TOKEN \
  -e PATROL_AUTO_DISCOVER \
  ghcr.io/chrisdietr/coolify-patrol:latest
```

### 2. Deploy in Coolify

1. Create a new **Docker** service in Coolify
2. Use image: `ghcr.io/chrisdietr/coolify-patrol:latest`
3. Set environment variables in Coolify UI:
   ```
   COOLIFY_URL=http://localhost:8000
   COOLIFY_TOKEN=your-api-token-here  
   PATROL_AUTO_DISCOVER=true
   ```
4. Expose port 8080 for health checks
5. Deploy! 🚀

### 3. Advanced: YAML Configuration (Optional)

For complex setups, you can still use YAML files:

```bash
cp patrol.yaml.example patrol.yaml
# Edit patrol.yaml, then:
docker run --rm \
  -v $(pwd)/patrol.yaml:/config/patrol.yaml:ro \
  ghcr.io/chrisdietr/coolify-patrol:latest
```

## Configuration

Coolify Patrol supports **two configuration methods**:

### Method 1: Environment Variables (Recommended for Coolify)

Perfect for Coolify's UI-based configuration:

```bash
# Required
COOLIFY_URL=http://localhost:8000           # Your Coolify server URL
COOLIFY_TOKEN=your-api-token-here          # API token from Coolify

# Choose one app discovery method:
PATROL_AUTO_DISCOVER=true                  # Auto-find all apps (recommended)

# OR specify exact apps:
PATROL_APPS="n8n:abc123:n8nio/n8n:auto-minor;postgres:def456:postgres:auto-patch:17"

# Optional settings:
PATROL_SCHEDULE="*/15 * * * *"             # Cron schedule (takes priority over interval)
PATROL_INTERVAL=15m                        # Check frequency (used if no schedule)
PATROL_POLICY=auto-patch                   # Default policy  
PATROL_COOLDOWN=1h                         # Wait between updates
PATROL_EXCLUDE_PATTERNS="-alpha,-beta,-rc" # Skip prerelease tags
PATROL_DRY_RUN=false                       # Test mode
PATROL_PORT=8080                           # Health check port
```

### Method 2: YAML Configuration (Advanced)

See [`patrol.yaml.example`](patrol.yaml.example) for a complete YAML example.
Environment variables override YAML settings when both are present.

### Update Policies

- **`auto-patch`** (default) - Only patch updates (1.2.3 → 1.2.4) - **SAFEST**
- **`auto-minor`** - Patch and minor updates (1.2.x → 1.3.0)  
- **`auto-all`** - All updates including major versions (use with caution)
- **`notify-only`** - Log available updates but don't apply them

### Auto-Discovery

When `PATROL_AUTO_DISCOVER=true`, Patrol automatically discovers all applications from Coolify and applies default policies. Applications with `latest` tags are skipped with a warning.

To preview discovered apps:

```bash
coolify-patrol discover
```

### Compact App Format

For `PATROL_APPS`, use: `"name:uuid:image[:policy[:pin]]"` separated by semicolons:

```bash
# Basic format
PATROL_APPS="app1:uuid1:image1;app2:uuid2:image2"

# With policies
PATROL_APPS="n8n:abc123:n8nio/n8n:auto-minor;postgres:def456:postgres:auto-patch"

# With policies and pins
PATROL_APPS="postgres:def456:postgres:auto-patch:17;redis:ghi789:redis:auto-minor:7"
```

### Cron Scheduling

Use `PATROL_SCHEDULE` for flexible timing with standard cron syntax:

```bash
# Every 15 minutes
PATROL_SCHEDULE="*/15 * * * *"

# Daily at 3 AM
PATROL_SCHEDULE="0 3 * * *"

# Every 6 hours
PATROL_SCHEDULE="0 */6 * * *"

# Twice daily (9 AM and 9 PM)
PATROL_SCHEDULE="0 9,21 * * *"

# Weekdays at 9 AM
PATROL_SCHEDULE="0 9 * * 1-5"
```

**Cron Format**: `minute hour day-of-month month day-of-week`
- **PATROL_SCHEDULE** takes priority over **PATROL_INTERVAL** when both are set

## CLI Usage

```bash
coolify-patrol [flags] [command]

Flags:
  --config <path>       Path to patrol.yaml (default: /config/patrol.yaml)
  --dry-run             Log what would be updated without making changes
  --once                Run one check cycle and exit
  --interval <duration> Override check interval (e.g., 5m, 1h)
  --log-format <fmt>    Log format: json (default) | text
  --port <port>         HTTP server port (default: 8080)
  --version             Print version and exit

Commands:
  check                 Run one check cycle (same as --once)
  status                Print current status of all watched apps
  discover              List all Coolify apps and suggest config
```

### Examples

```bash
# Run once and exit (good for cron jobs)
coolify-patrol --once

# Dry run to see what would be updated
coolify-patrol --dry-run --once

# Override check interval
coolify-patrol --interval 5m

# Check status
coolify-patrol status

# Discover applications for configuration
coolify-patrol discover > suggested-config.yaml
```

## API Endpoints

- `GET /health` - Health check endpoint
- `GET /status` - Detailed status of all watched applications

Example status response:

```json
{
  "status": "running",
  "last_check": "2026-02-22T20:30:00Z",
  "apps": [
    {
      "name": "n8n",
      "uuid": "app-uuid",
      "image": "n8nio/n8n",
      "current_tag": "1.63.1",
      "latest_tag": "1.63.2", 
      "policy": "auto-patch",
      "update_needed": true,
      "last_check": "2026-02-22T20:30:00Z",
      "next_check": "2026-02-22T20:45:00Z"
    }
  ]
}
```

## Building

### Prerequisites

- Go 1.24+
- Docker (for container builds)

### Build Binary

```bash
go build -o coolify-patrol ./cmd/patrol
```

### Build Docker Image

```bash
docker build -t coolify-patrol .
```

### Run Tests

```bash
go test ./...
```

## Security Considerations

- **API Token**: Store your Coolify API token securely (environment variable or secrets management)
- **Network Access**: Patrol needs access to Coolify API and Docker registries
- **Update Policies**: Default `auto-patch` is conservative; major/minor updates require explicit opt-in
- **Container Security**: Runs as non-root user in container

## Troubleshooting

### Common Issues

1. **Authentication Failed**
   ```
   Failed to connect to Coolify: authentication failed - check your API token
   ```
   Ensure your `COOLIFY_API_TOKEN` is valid and has sufficient permissions.

2. **Rate Limited**
   ```
   Rate limited by Docker Hub
   ```
   Consider authenticating to Docker Hub or reducing check frequency.

3. **App Not Found**
   ```
   Application not found: uuid
   ```
   Check that the application UUID exists in Coolify and hasn't been recreated.

### Debug Logging

Use text format logs for easier reading during development:

```bash
coolify-patrol --log-format text --dry-run --once
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) file for details.