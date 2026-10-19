# Recourse — Financial Dispute Resolution API

> High-performance Go microservice · GraphQL API · JWT RBAC · pgx/v5 PostgreSQL · chi router

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![GraphQL](https://img.shields.io/badge/GraphQL-API-E10098?logo=graphql)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql)
![License](https://img.shields.io/badge/license-MIT-green)

---

## Overview

**Recourse** is a production-ready Go microservice for managing financial transaction dispute cases. It exposes a unified GraphQL API with full JWT authentication, hierarchical role-based access control (RBAC), and native pgx/v5 PostgreSQL persistence with connection pooling.

### Key Features

| Feature | Detail |
|---|---|
| **GraphQL API** | Full schema with queries & mutations, embedded GraphiQL IDE |
| **Authentication** | HS256 JWT with configurable expiry, bcrypt password hashing |
| **RBAC** | 4-tier role hierarchy (ADMIN → ANALYST → AUDITOR → USER) |
| **Database** | pgx/v5 native driver, pool of 25 conns, prepared statements, 30s statement timeouts |
| **Router** | go-chi/chi v5 with CORS, request tracing IDs, timeout middleware |
| **Logging** | Structured JSON via `log/slog` with trace_id, user_id, role propagation |
| **Container** | Multi-stage Docker build → distroless image (~15MB) |
| **Docs Frontend** | Premium interactive docs page at `GET /` |

---

## Architecture

```
Client
  │
  ▼
chi Router  (JWT injection · CORS · request-ID · timeout · recoverer)
  │
  ├── POST /graphql  ──► GraphQL Execution Engine
  │                        │
  │                        ├── Query Resolvers  ──► UserRepo / DisputeRepo
  │                        └── Mutation Resolvers ──► UserRepo / DisputeRepo
  │                                                       │
  │                                                 pgxpool (PostgreSQL)
  │
  ├── GET  /graphql  ──► Embedded GraphiQL IDE
  ├── GET  /health · /ready  ──► Health endpoints
  ├── POST /api/v1/auth/*    ──► REST convenience (delegates to GraphQL)
  ├── GET  /api/v1/disputes* ──► REST read endpoints
  └── GET  /          ──► Documentation frontend
```

### Project Layout

```
recourse/
├── cmd/server/main.go          # Entrypoint — wires all dependencies
├── internal/
│   ├── api/router.go           # chi router, all middleware, route registration
│   ├── auth/
│   │   ├── jwt.go              # JWT service (generate, validate, middleware)
│   │   └── jwt_test.go         # Unit tests
│   ├── config/config.go        # Environment-based configuration
│   ├── db/
│   │   ├── db.go               # pgxpool setup and lifecycle
│   │   ├── user_repo.go        # User CRUD with pgx prepared statements
│   │   └── dispute_repo.go     # Dispute CRUD with status transitions
│   ├── graph/
│   │   ├── handler.go          # GraphQL HTTP handler + execution engine
│   │   ├── resolver.go         # Root resolver, GQL type definitions
│   │   └── resolvers.go        # All query & mutation implementations
│   ├── logger/logger.go        # Structured JSON logging with context propagation
│   ├── models/models.go        # Domain types (User, Dispute, AuthClaims)
│   └── rbac/
│       ├── rbac.go             # Role hierarchy enforcement
│       └── rbac_test.go        # Unit tests
├── migrations/
│   └── 000001_create_tables.up.sql
├── web/index.html              # Interactive documentation frontend
├── Dockerfile                  # Multi-stage production build
└── docker-compose.yml          # Full stack (API + PostgreSQL)
```

---

## Quickstart

### Docker Compose (Recommended)

```bash
# Start everything — PostgreSQL + Recourse API
docker compose up -d

# Watch logs
docker compose logs -f recourse

# Open interactive docs
open http://localhost:8083

# Open GraphiQL
open http://localhost:8083/graphql
```

### Native (Go 1.25+)

```bash
# Set up PostgreSQL
psql -c "CREATE DATABASE recourse_db;"
psql recourse_db -f migrations/000001_create_tables.up.sql

# Configure environment
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/recourse_db?sslmode=disable"
export JWT_SECRET="your-secret-key-min-32-chars"

# Run
go run ./cmd/server
```

---

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8083` | HTTP server port |
| `SERVICE_NAME` | `recourse-backend` | Service identifier in logs |
| `DATABASE_URL` | `postgres://postgres:...` | PostgreSQL connection string |
| `DB_MAX_CONNS` | `25` | pgxpool max connections |
| `DB_MIN_CONNS` | `5` | pgxpool min connections |
| `JWT_SECRET` | *(insecure default)* | **Always override in production** |
| `JWT_EXPIRATION_HOURS` | `24` | Token validity duration |
| `REQUEST_TIMEOUT_SEC` | `5` | Per-request context timeout |
| `LOG_LEVEL` | `INFO` | `DEBUG` · `INFO` · `WARN` · `ERROR` |

---

## API Reference

### Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/graphql` | Conditional | Main GraphQL endpoint |
| `GET` | `/graphql` | None | Embedded GraphiQL IDE |
| `GET` | `/health` · `/ready` | None | Health/readiness |
| `POST` | `/api/v1/auth/register` | None | Register user |
| `POST` | `/api/v1/auth/login` | None | Login |
| `GET` | `/api/v1/disputes` | JWT | List disputes |
| `GET` | `/api/v1/disputes/{id}` | JWT | Get dispute |
| `GET` | `/` | None | Docs frontend |

### Roles & Permissions

```
ADMIN   (100) — disputes:*, users:read, users:manage
ANALYST  (50) — disputes:read, disputes:write, disputes:resolve, users:read
AUDITOR  (30) — disputes:read, users:read
USER     (10) — disputes:read, disputes:write
```

### GraphQL Operations

#### Register + Login
```graphql
mutation Register($input: RegisterInput!) {
  register(input: $input) { token expiresIn user { id email role } }
}

mutation Login($input: LoginInput!) {
  login(input: $input) { token expiresIn }
}
```

#### Disputes
```graphql
# Create (any auth)
mutation CreateDispute($input: CreateDisputeInput!) {
  createDispute(input: $input) { id title status createdAt }
}

# List with optional filter
query Disputes($status: DisputeStatus) {
  disputes(status: $status) { id title amount currency status }
}

# Update status (ANALYST+)
mutation UpdateDispute($id: ID!, $input: UpdateDisputeInput!) {
  updateDispute(id: $id, input: $input) { id status resolutionNotes }
}

# Delete (ADMIN only)
mutation DeleteDispute($id: ID!) { deleteDispute(id: $id) }
```

#### Users
```graphql
query Me { me { id email fullName role } }
query Users { users { id email role createdAt } }  # ANALYST+
mutation UpdateUserRole($userId: ID!, $role: Role!) {
  updateUserRole(userId: $userId, role: $role) { id role }  # ADMIN
}
```

---

## Testing

```bash
# Run all tests with race detector
go test -race -count=1 -v ./internal/...

# Individual packages
go test -race ./internal/auth/...
go test -race ./internal/rbac/...
```

---

## Database Schema

```sql
-- Users table
CREATE TABLE users (
    id           VARCHAR(36) PRIMARY KEY,
    email        VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name    VARCHAR(255) NOT NULL,
    role         VARCHAR(50)  NOT NULL DEFAULT 'USER',
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Disputes table
CREATE TABLE disputes (
    id               VARCHAR(36) PRIMARY KEY,
    title            VARCHAR(255) NOT NULL,
    description      TEXT NOT NULL,
    amount           NUMERIC(12, 2) NOT NULL,
    currency         VARCHAR(10) NOT NULL DEFAULT 'USD',
    status           VARCHAR(50) NOT NULL DEFAULT 'OPEN',
    created_by_id    VARCHAR(36) NOT NULL REFERENCES users(id),
    assignee_id      VARCHAR(36) REFERENCES users(id),
    resolution_notes TEXT,
    created_at       TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

---

## Production Notes

- **Change `JWT_SECRET`** — the default is insecure
- pgx pool is sized for **25 max connections** — tune `DB_MAX_CONNS` for your Postgres `max_connections`
- The distroless image runs as `nonroot:nonroot` — no shell, minimal attack surface
- Request tracing: every request gets an `X-Request-ID` header (generated if not supplied by caller)
- All log output is **structured JSON** — compatible with Datadog, GCP Logging, ELK stack

---

*Part of the **igeTayor** microservices portfolio — alongside **fickle** (WebSocket hub) and **telex** (event streaming worker).*
