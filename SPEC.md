# RFC-0006: Coolify Patrol — Automated Docker Image Update Service

**Status:** Draft
**Author:** TeleMac
**Created:** 2026-02-22
**Target:** Open source (public repo)

## Abstract

Coolify Patrol is a lightweight service that monitors Docker image registries for new releases and automatically updates deployed applications on Coolify servers via the Coolify API. It applies semver-aware update policies (auto-patch, auto-minor, notify-only) to balance security with stability. It runs as a Docker container deployed within Coolify itself.

## Motivation

Coolify does not auto-update externally sourced Docker images. Users must manually bump image tags. With increasing CVE velocity in popular software (n8n, Postgres, Redis, etc.), this creates a growing security gap. Most CVE fixes ship as patch releases that are safe to auto-apply.

Existing solutions (Watchtower, Diun) don't integrate with Coolify's API — they restart containers directly, bypassing Coolify's deployment pipeline, health checks, and notification system.

## Requirements

### MUST

1. **M1**: Poll Docker registries (Docker Hub, GHCR) for new image tags at a configurable interval.
2. **M2**: Compare discovered tags with currently deployed tags via the Coolify API (`GET /api/v1/applications/{uuid}`).
3. **M3**: Parse image tags as semver where possible, falling back to lexicographic comparison for non-semver tags.
4. **M4**: Support per-application update policies:
   - `auto-patch`: Automatically update patch versions (e.g., 1.2.3 → 1.2.4). Default.
   - `auto-minor`: Automatically update patch and minor versions (e.g., 1.2.x → 1.3.0).
   - `notify-only`: Never auto-update, only log that an update is available.
   - `auto-all`: Automatically update all versions including major (use with caution).
5. **M5**: Update applications via the Coolify API (`PATCH /api/v1/applications/{uuid}` to set new image tag, then `POST /api/v1/applications/{uuid}/restart` to trigger redeploy).
6. **M6**: Support a YAML configuration file defining watched applications and their policies.
7. **M7**: Support major version pinning (e.g., `pin: "17"` for postgres:17.x — never cross to 18.x).
8. **M8**: Log all actions (checks, updates, skips, errors) in structured JSON format.
9. **M9**: Expose a `/health` endpoint for Coolify health checks.
10. **M10**: Run as a single Docker container with no external dependencies beyond the Coolify API and Docker registry APIs.
11. **M11**: Authenticate to Coolify via API token passed as environment variable.
12. **M12**: Handle rate limiting from Docker Hub (100 pulls/6h for anonymous, 200 for authenticated).
13. **M13**: Provide a dry-run mode (`--dry-run` or `PATROL_DRY_RUN=true`) that logs what would be updated without making changes.

### SHOULD

14. **S1**: Support authenticated Docker registry access (Docker Hub tokens, GHCR tokens) for private images.
15. **S2**: Support a cooldown period per application after an update (e.g., don't update again within 1 hour even if a newer tag appears).
16. **S3**: Provide a `/status` endpoint returning JSON with all watched apps, current vs latest tags, and last check time.
17. **S4**: Support filtering tags by pattern (e.g., ignore `-alpha`, `-rc`, `-beta` suffixes by default).
18. **S5**: Support Coolify's `docker-compose` type services (not just standalone applications).
19. **S6**: Detect when a deployed image uses `latest` tag and warn (since semver comparison is impossible).
20. **S7**: Cache registry responses to minimize API calls between intervals.
21. **S8**: Support webhook notifications (generic HTTP POST) for integration with external systems.

### MAY

22. **Y1**: Integrate with OSV.dev or GitHub Advisory Database to flag CVE-affected versions.
23. **Y2**: Support rollback — if Coolify reports deployment failure, revert to previous tag.
24. **Y3**: Provide a web UI dashboard showing update history and pending updates.
25. **Y4**: Support watching Helm charts or docker-compose files in git repos.

## Architecture

```
┌─────────────────────────────────────────┐
│              Coolify Server              │
│                                          │
│  ┌──────────┐  ┌──────────┐  ┌────────┐ │
│  │  n8n     │  │ postgres │  │ redis  │ │
│  │  :5678   │  │  :5432   │  │ :6379  │ │
│  └──────────┘  └──────────┘  └────────┘ │
│                                          │
│  ┌──────────────────────────────────┐    │
│  │       coolify-patrol             │    │
│  │                                  │    │
│  │  ┌─────────┐   ┌─────────────┐  │    │
│  │  │ Watcher │──►│ Coolify API │  │    │
│  │  │ (cron)  │   │ (update +   │  │    │
│  │  └────┬────┘   │  restart)   │  │    │
│  │       │        └─────────────┘  │    │
│  │       ▼                         │    │
│  │  ┌──────────┐                   │    │
│  │  │ Registry │                   │    │
│  │  │ API      │                   │    │
│  │  │ (check)  │                   │    │
│  │  └──────────┘                   │    │
│  │                                  │    │
│  │  :8080 /health /status           │    │
│  └──────────────────────────────────┘    │
│                                          │
│  ┌──────────────────┐                    │
│  │  Coolify Engine   │                    │
│  │  (notifications,  │                    │
│  │   deploy, health) │                    │
│  └──────────────────┘                    │
└─────────────────────────────────────────┘
```

### Components

1. **Watcher**: Runs on a timer (default 15 minutes). For each configured app:
   - Fetches latest tags from registry
   - Compares with deployed tag (from Coolify API)
   - Applies semver policy
   - Triggers update if policy allows

2. **Registry Client**: Talks to Docker Hub API v2 and GHCR API.
   - `GET /v2/{namespace}/{repo}/tags/list` for tag enumeration
   - Handles pagination, rate limiting, auth tokens
   - Filters out pre-release tags by default

3. **Coolify Client**: Talks to Coolify API v1.
   - `GET /api/v1/applications` — list all apps
   - `GET /api/v1/applications/{uuid}` — get current config (includes image tag)
   - `PATCH /api/v1/applications/{uuid}` — update image tag
   - `POST /api/v1/applications/{uuid}/restart` — trigger redeploy
   - Auth via `Authorization: Bearer <token>` header

4. **HTTP Server**: Minimal health/status endpoints.
   - `GET /health` → `{"ok": true}`
   - `GET /status` → watched apps, versions, last check, pending updates

### Configuration

```yaml
# patrol.yaml
coolify:
  url: http://localhost:8000  # or https://coolify.example.com
  token: ${COOLIFY_API_TOKEN}  # env var substitution

defaults:
  policy: auto-patch
  interval: 15m
  cooldown: 1h
  exclude_patterns:
    - "-alpha"
    - "-beta"
    - "-rc"
    - "-dev"
    - "-nightly"

apps:
  - name: n8n
    uuid: app-uuid-from-coolify
    image: n8nio/n8n
    # policy: auto-patch (inherited from defaults)

  - name: plausible
    uuid: app-uuid-2
    image: ghcr.io/plausible/community-edition
    policy: auto-minor

  - name: postgres
    uuid: app-uuid-3
    image: postgres
    pin: "17"  # stay within 17.x.x
    policy: auto-patch

  - name: redis
    uuid: app-uuid-4
    image: redis
    pin: "7"
    policy: auto-patch

  - name: grafana
    uuid: app-uuid-5
    image: grafana/grafana
    policy: notify-only  # just log, don't touch
```

### Auto-Discovery Mode

If no `apps` section is provided, Patrol SHOULD be able to auto-discover applications from the Coolify API and apply the default policy to all of them. This requires:

- `GET /api/v1/applications` to list all apps
- Extract image references from each app's configuration
- Skip apps with `latest` tag (warn instead)
- Skip Coolify's own internal services

### Semver Logic

```
Given: deployed=1.2.3, latest=1.2.5, policy=auto-patch
→ Update (patch increment only)

Given: deployed=1.2.3, latest=1.3.0, policy=auto-patch
→ Skip (minor increment, policy is patch-only)

Given: deployed=1.2.3, latest=1.3.0, policy=auto-minor
→ Update (minor increment allowed)

Given: deployed=17.2.1, latest=18.0.0, pin="17"
→ Skip (major crosses pin boundary)

Given: deployed=17.2.1, latest=17.3.0, pin="17", policy=auto-patch
→ Skip (minor increment, policy is patch-only, pin respected)

Given: deployed=17.2.1, latest=17.2.2, pin="17", policy=auto-patch
→ Update (patch within pinned major)
```

## Edge Cases

1. **Non-semver tags** (e.g., `latest`, `stable`, `bookworm`): Compare by image digest. If digest differs, treat as "update available". Policy `notify-only` is forced for non-semver tags unless user explicitly sets `auto-all`.

2. **Tag disappeared from registry**: Log warning, do not modify deployment. Tag may have been yanked for security reasons — flag for human review.

3. **Coolify API unreachable**: Log error, retry on next interval. Do not crash.

4. **Registry rate limited**: Back off exponentially. Log rate limit headers. Spread checks across the interval to avoid bursts.

5. **Concurrent updates**: If two apps need updating simultaneously, process sequentially with a configurable delay between updates (default 30s).

6. **Coolify deployment fails**: Patrol does not own rollback (Coolify handles this). Log the failure. Cooldown prevents immediate re-attempt.

7. **App UUID changes** (user recreates app in Coolify): Patrol will get 404 from API. Log error, skip app, continue.

8. **Multiple tags for same digest**: Pick the highest semver tag. Ignore duplicates.

9. **Private registries**: Require auth config. If auth fails, log error and skip (don't fall back to anonymous).

10. **Self-update**: Patrol can watch its own image. It will trigger a Coolify redeploy of itself, which Coolify handles gracefully (rolling restart).

## CLI

```
coolify-patrol [flags]

Flags:
  --config <path>       Path to patrol.yaml (default: /config/patrol.yaml)
  --dry-run             Log what would be updated without making changes
  --once                Run one check cycle and exit (for external cron)
  --interval <duration> Override check interval (e.g., 5m, 1h)
  --log-format <fmt>    Log format: json (default) | text
  --port <port>         HTTP server port (default: 8080)
  --version             Print version and exit

Commands:
  check                 Run one check cycle (same as --once)
  status                Print current status of all watched apps
  discover              List all Coolify apps and suggest config
```

## Docker

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /coolify-patrol ./cmd/patrol

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /coolify-patrol /usr/local/bin/coolify-patrol
EXPOSE 8080
ENTRYPOINT ["coolify-patrol"]
```

## Security Considerations

- **API Token**: Coolify API token has full access. Patrol only needs read + update on applications. If Coolify adds scoped tokens in the future, use minimal scope.
- **Registry Auth**: Tokens stored in config or env vars. Never logged.
- **Network**: Patrol should run on Coolify's internal Docker network to access the API via localhost. External access to Patrol's HTTP server is optional.
- **Update Policy**: Default `auto-patch` is conservative. Major and minor updates require explicit opt-in.

Conformance criteria are tested separately.
