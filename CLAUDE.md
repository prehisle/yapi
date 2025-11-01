# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

yapi is a Go-based intelligent LLM proxy gateway that provides unified entry point, dynamic rule routing, and admin management capabilities with streaming forwarding support for upstream model APIs.

## Development Commands

### Go Backend

**Build:**
```bash
go build ./cmd/gateway
```

**Run tests:**
```bash
go test ./...
```

**Run single test:**
```bash
go test ./pkg/rules -run TestService_CreateRule
```

**Linting and formatting:**
```bash
# First time setup
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

make lint           # Run golangci-lint
make test           # Run all tests
make verify         # Full verification with Docker Compose
golangci-lint run ./...  # Direct linting
```

**Run integration tests (requires Docker Compose):**
```bash
go test ./internal/integration_test -tags compose_test
```

### Frontend Admin UI

Located in `web/admin/` directory:

```bash
cd web/admin
npm install        # Install dependencies
npm run dev        # Development server (proxies to localhost:8080)
npm run build      # Build for production
npm run lint       # ESLint check
npm run test:e2e   # End-to-end tests (requires Docker Compose)

# Environment variables:
# VITE_API_BASE_URL - Override API base URL (default: http://localhost:8080)
# PLAYWRIGHT_API_BASE_URL - Override E2E testing API URL
```

**E2E Testing:**
- Requires running `docker compose -f deploy/docker-compose.yml up -d` first
- Uses Playwright with browser automation
- Configure via `PLAYWRIGHT_API_BASE_URL`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`
- Run `npx playwright install --with-deps chromium` before first execution
- Frontend build required before running E2E tests

### Docker Development

**Start development environment:**
```bash
docker compose -f deploy/docker-compose.yml up -d
```

**Start with monitoring (Prometheus + Grafana):**
```bash
docker compose -f deploy/docker-compose.monitoring.yml up -d
```

**Port mappings:**
- Gateway: 8080
- Redis: 9108 (for testing)
- PostgreSQL: 9109 (for testing)
- HTTPbin: 9107 (for testing)
- Prometheus: 9090
- Grafana: 3000 (admin/admin)

## Architecture Overview

### Core Components

1. **Gateway Server** (`cmd/gateway/main.go`): Main application entry point
   - Initializes configuration, database, Redis, and routing
   - Handles graceful shutdown and environment loading
   - Seeds default rules on first startup

2. **Proxy Handler** (`internal/proxy/handler.go`): Request routing and forwarding
   - Rule-based request matching against path, method, headers
   - Supports path rewriting, header manipulation, JSON body transformation
   - Reverse proxy with metrics collection and structured logging

3. **Admin API** (`internal/admin/`): Management interface
   - RESTful API for rule CRUD operations under `/admin` prefix
   - Authentication via Basic Auth or JWT Bearer tokens
   - Health checks and login endpoints

4. **Rules Engine** (`pkg/rules/`): Core business logic
   - Rule matching, validation, and storage abstraction
   - PostgreSQL persistence with GORM, Redis caching and pub/sub
   - JSON path manipulation for request body transformation

5. **Account Management** (`pkg/accounts/`): Multi-tenant API management
   - User entities with metadata support and API key generation
   - Upstream credential management per user with JSON endpoint storage
   - API key to upstream credential bindings with transaction consistency
   - Context-aware rule matching based on user identity and bindings

### Key Patterns

- **Account-Aware Routing**: Rules can match based on API keys, user IDs, metadata, and upstream bindings
- **Rule Matching**: Priority-based rule evaluation (higher priority = earlier evaluation)
- **Multi-layer Caching**: Redis cache with in-memory fallback, pub/sub invalidation
- **Graceful Degradation**: Falls back to in-memory store when Redis unavailable
- **Structured Logging**: All operations emit slog logs with request IDs for tracing
- **Metrics**: Prometheus metrics for upstream requests and admin operations
- **Context Propagation**: Account context flows through middleware for authorization and routing

### Database Schema

Uses GORM auto-migration for rule and account storage. Key models include:

**Rules:**
- ID, Priority, Enabled status
- Matcher (path prefix, methods, headers, account context fields)
- Actions (target URL, headers, authorization, JSON overrides)
- Created/Updated timestamps

**Account Management:**
- **Users**: ID, Name, Description, JSON metadata, soft delete
- **APIKeys**: ID, bcrypt hash, user relationship, 8-char prefix, timestamps
- **UpstreamCredentials**: ID, user relationship, provider, JSON endpoints, metadata
- **UserAPIKeyBindings**: API key to upstream credential mappings with timestamps

### Configuration

Environment-based configuration (see `.env.example` and `.env.local.example`):
- `GATEWAY_PORT`: Server port (default: 8080)
- `DATABASE_DSN`: PostgreSQL connection string
- `REDIS_ADDR`: Redis server address (default: localhost:6379)
- `REDIS_CHANNEL`: Rule sync channel (default: rules:sync)
- `REDIS_MAINT_NOTIFICATIONS_MODE`: Redis maintenance mode (disabled/auto/enabled)
- `ADMIN_USERNAME/PASSWORD`: Basic auth credentials
- `ADMIN_TOKEN_SECRET`: JWT signing secret
- `ADMIN_TOKEN_TTL`: JWT expiration time (default: 30m)
- `ADMIN_ALLOWED_ORIGINS`: CORS allowed origins (comma-separated)
- `UPSTREAM_BASE_URL`: Default fallback upstream

## Security Considerations

### Authentication
- **Basic Auth**: Configurable via `ADMIN_USERNAME`/`ADMIN_PASSWORD`
- **JWT Tokens**: Short-lived tokens with configurable TTL via `ADMIN_TOKEN_SECRET`
- **CORS**: Configurable origin whitelisting via `ADMIN_ALLOWED_ORIGINS`
- **API Key Authentication**: Automatic API key validation and upstream credential injection

### JSON Processing Security
- JSON rewrite operations use `tidwall/sjson` library
- Only processes `Content-Type: application/json` requests
- Errors logged with `X-YAPI-Body-Rewrite-Error` header for audit tracking

### Environment Security
- Use `.env.local` for local development (never commit credentials)
- Production environments should use secrets management
- Rotate JWT secrets regularly

## Testing Strategy

- Unit tests for individual components using testify
- Integration tests with `-tags compose_test` requiring Docker Compose
- Golden file testing in `testdata/` directory
- End-to-end frontend tests using Playwright in `web/admin/tests/`
- Health check and regression testing via `scripts/verify_gateway.sh`

## Environment Management

### Local Development
1. Copy `.env.local.example` to `.env.local` and configure
2. Use strong random credentials for production
3. Generate JWT secrets: `openssl rand -hex 32`
4. Configure CORS origins as needed

### Environment Variables
- Use separate files for different environments
- Never commit `.env.local` to version control
- Production should use secrets management systems

## Development Workflow

1. Copy `.env.local.example` to `.env.local` and configure
2. Run `make lint` before committing
3. Use `make verify` for full integration testing
4. Follow Go formatting standards (gofmt/gofumpt)
5. Test naming convention: `Test<Component>_<Scenario>`
6. Account service is optional - gateway operates with reduced functionality without database

## Observability

- **Metrics**: `/metrics` endpoint with Prometheus data
- **Logging**: Structured JSON logs with request IDs
- **Health**: `/admin/healthz` endpoint for service status
- **Tracing**: Request IDs flow through entire proxy chain