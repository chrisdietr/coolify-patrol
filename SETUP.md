# Setup Guide

## 1. Get Coolify API Token

1. Log into your Coolify instance
2. Go to **Settings** → **API Tokens**  
3. Click **Create New Token**
4. Give it a descriptive name like "coolify-patrol"
5. Copy the generated token (you'll need this)

## 2. Configure via Environment Variables

The simplest approach for Coolify deployment is environment variables:

```bash
# Required settings
COOLIFY_URL=http://localhost:8000
COOLIFY_TOKEN=your-api-token-here

# Auto-discover all applications (recommended)
PATROL_AUTO_DISCOVER=true

# Optional: customize behavior
PATROL_SCHEDULE="*/15 * * * *"  # Cron schedule (every 15 min)
# OR
PATROL_INTERVAL=15m             # Simple interval (if no schedule)
PATROL_POLICY=auto-patch        # Default policy for all apps
PATROL_COOLDOWN=1h              # Wait between updates
```

### Alternative: Specific Apps

Instead of auto-discovery, specify exact applications:

```bash
# Format: "name:uuid:image[:policy[:pin]]" separated by semicolons
PATROL_APPS="n8n:abc123:n8nio/n8n:auto-minor;postgres:def456:postgres:auto-patch:17"
```

### Optional: YAML Configuration

For advanced setups, you can still use YAML files (see `patrol.yaml.example`). Environment variables will override YAML settings.

## 3. Find App UUIDs

You can find UUIDs in:

- **Coolify dashboard URL**: `https://coolify.example.com/application/uuid-here`
- **Auto-discovery**: Run `coolify-patrol discover` to see all apps

## 4. Deploy in Coolify

### Simple Docker Service (Recommended)

1. **Create new Git Repository service** in Coolify
2. **Set repository:** `https://github.com/chrisdietr/coolify-patrol`
3. **Build from the included Dockerfile**
4. **Add environment variables** in Coolify UI:
   ```
   COOLIFY_URL=http://localhost:8000
   COOLIFY_TOKEN=your-api-token-here
   PATROL_AUTO_DISCOVER=true
   PATROL_INTERVAL=15m
   PATROL_POLICY=auto-patch
   ```
5. **Expose port 8080** for health checks
6. **Deploy!** 🚀

### Advanced: Docker Compose

For custom configurations, use docker-compose:

```yaml
services:
  coolify-patrol:
    build:
      context: .
    environment:
      - COOLIFY_URL=http://localhost:8000
      - COOLIFY_TOKEN=your-api-token-here
      - PATROL_AUTO_DISCOVER=true
      - PATROL_INTERVAL=15m
      - PATROL_POLICY=auto-patch
    ports:
      - "8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

No volumes or config files needed! ✨

## 5. Auto-Discovery Mode

If you don't want to configure specific apps, Patrol can auto-discover all your Coolify applications:

1. Remove the `apps:` section from your `patrol.yaml` (or leave it empty)
2. Patrol will automatically monitor all apps with semantic version tags
3. Apps using `latest` tag will be skipped with a warning

## 6. Verify Setup

### Check Health
```bash
curl http://your-coolify-server:8080/health
```

### Check Status  
```bash
curl http://your-coolify-server:8080/status
```

### View Logs
Check Coolify's service logs to see Patrol's activity:

```json
{
  "level": "info",
  "msg": "Version check completed",
  "app": "n8n",
  "current_tag": "1.63.1",
  "latest_tag": "1.63.2",
  "update_needed": true,
  "policy": "auto-patch"
}
```

## 7. Testing

Before going live, test with dry-run mode:

1. Set environment variable: `PATROL_DRY_RUN=true`
2. Or run manually: `coolify-patrol --dry-run --once`

This will log what updates would be made without actually applying them.

## Troubleshooting

### Authentication Issues
```
Failed to connect to Coolify: authentication failed
```
- Verify your API token is correct
- Check that API access is enabled in Coolify settings

### App Not Found
```  
Application not found: uuid
```
- Verify the UUID exists in Coolify
- App may have been deleted and recreated (UUID changes)

### Rate Limiting
```
Rate limited by Docker Hub  
```
- Reduce check frequency in configuration
- Consider authenticating to Docker Hub for higher limits

### Debug Mode
For more verbose output during troubleshooting:

```bash
coolify-patrol --log-format text --dry-run --once
```