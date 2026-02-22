# Coolify Patrol Demo

This document demonstrates the environment variable configuration in action.

## Environment-Only Configuration Demo

### 1. Auto-Discovery Mode
```bash
# Set up environment variables
export COOLIFY_URL="http://localhost:8000"
export COOLIFY_TOKEN="your-api-token-here"
export PATROL_AUTO_DISCOVER=true
export PATROL_SCHEDULE="*/10 * * * *"  # Every 10 minutes via cron
export PATROL_POLICY=auto-patch

# Run coolify-patrol (no config file needed!)
coolify-patrol --once --dry-run
```

### 2. Specific Apps Mode
```bash
# Set up with specific applications
export COOLIFY_URL="http://localhost:8000"
export COOLIFY_TOKEN="your-api-token-here"
export PATROL_APPS="n8n:abc-123:n8nio/n8n:auto-minor;postgres:def-456:postgres:auto-patch:17"

# Run coolify-patrol
coolify-patrol --once --dry-run
```

### 3. Docker Container Deployment
```bash
# Run as Docker container with environment variables
docker run -d \
  --name coolify-patrol \
  -e COOLIFY_URL=http://localhost:8000 \
  -e COOLIFY_TOKEN=your-api-token-here \
  -e PATROL_AUTO_DISCOVER=true \
  -e PATROL_INTERVAL=15m \
  -e PATROL_DRY_RUN=false \
  -p 8080:8080 \
  ghcr.io/chrisdietr/coolify-patrol:latest
```

### 4. Coolify UI Configuration
When deploying in Coolify, simply add these environment variables in the UI:

**Required:**
- `COOLIFY_URL` = `http://localhost:8000`
- `COOLIFY_TOKEN` = `your-api-token`

**Choose one:**
- `PATROL_AUTO_DISCOVER` = `true` (auto-find all apps)
- OR `PATROL_APPS` = `"app1:uuid1:image1;app2:uuid2:image2"`

**Optional:**
- `PATROL_INTERVAL` = `15m`
- `PATROL_POLICY` = `auto-patch`
- `PATROL_COOLDOWN` = `1h`
- `PATROL_DRY_RUN` = `false`

**That's it!** No YAML files, no volume mounts needed. ✨

## Configuration Priority

Environment variables **always override** YAML configuration:

1. **Environment Variables** (highest priority)
2. **YAML Configuration** (fallback)
3. **Built-in Defaults** (lowest priority)

## Examples

### Compact App Format Examples
```bash
# Single app
PATROL_APPS="n8n:abc123:n8nio/n8n"

# With policy
PATROL_APPS="n8n:abc123:n8nio/n8n:auto-minor"

# With policy and pin
PATROL_APPS="postgres:def456:postgres:auto-patch:17"

# Multiple apps
PATROL_APPS="n8n:abc123:n8nio/n8n:auto-minor;postgres:def456:postgres:auto-patch:17;redis:ghi789:redis"
```

### Policy Examples
```bash
# Conservative (default) - only patch updates
PATROL_POLICY=auto-patch

# Moderate - patch and minor updates
PATROL_POLICY=auto-minor

# Aggressive - all updates (use with caution)
PATROL_POLICY=auto-all

# Monitor only - log but don't update
PATROL_POLICY=notify-only
```

### Cron Schedule Examples
```bash
# Every 15 minutes
PATROL_SCHEDULE="*/15 * * * *"

# Every hour at minute 30
PATROL_SCHEDULE="30 * * * *"

# Daily at 3 AM
PATROL_SCHEDULE="0 3 * * *"

# Every 6 hours
PATROL_SCHEDULE="0 */6 * * *"

# Twice daily: 9 AM and 9 PM
PATROL_SCHEDULE="0 9,21 * * *"

# Business hours only: 9 AM-5 PM on weekdays
PATROL_SCHEDULE="0 9-17 * * 1-5"

# Weekly: Sunday at midnight
PATROL_SCHEDULE="0 0 * * 0"

# Monthly: 1st day at 2 AM
PATROL_SCHEDULE="0 2 1 * *"
```

**Cron Format**: `minute(0-59) hour(0-23) day(1-31) month(1-12) weekday(0-7)`
- Use `*` for "any value"
- Use `*/N` for "every N units"
- Use `N-M` for ranges
- Use `N,M,O` for lists

### Pin Examples
```bash
# Keep PostgreSQL within major version 17
PATROL_APPS="postgres:uuid:postgres:auto-patch:17"

# Keep Node.js within major version 20
PATROL_APPS="node-app:uuid:node:auto-minor:20"
```

## Health Checks

Access these endpoints for monitoring:

- **Health**: `http://localhost:8080/health`
- **Status**: `http://localhost:8080/status`

Example status response:
```json
{
  "status": "running",
  "last_check": "2026-02-22T20:30:00Z",
  "apps": [
    {
      "name": "n8n",
      "uuid": "abc123",
      "image": "n8nio/n8n",
      "current_tag": "1.63.1",
      "latest_tag": "1.63.2",
      "policy": "auto-patch",
      "update_needed": true,
      "last_check": "2026-02-22T20:30:00Z"
    }
  ]
}
```