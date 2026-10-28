# Senior Engineering Walkthrough: Recourse GraphQL Dispute Resolution API

> **Target Audience:** Principal Systems Engineers, FinTech Architects, Enterprise API Platform Leads.  
> **Key Architecture Highlights:** GraphQL API with Embedded GraphiQL IDE, 4-Tier Hierarchical RBAC, Native `pgx/v5` PostgreSQL Driver with Connection Pooling, Request Tracing Correlation IDs, Structured JSON Logging (`log/slog`).

---

## 1. Executive Summary & Problem Space

In enterprise banking and payment networks (Visa/Mastercard/Stripe chargeback flows), handling financial disputes requires multi-tenant data isolation, strict auditable role hierarchies (Users, Auditors, Analysts, Admins), and high-throughput query performance. REST APIs often suffer from over-fetching or under-fetching when rendering complex dispute timelines with linked evidence files, transaction ledgers, and analyst comments.

**Recourse** provides a production-grade GraphQL dispute resolution engine in Go:
- **Unified GraphQL Schema:** Clean type system with queries for dispute case files and mutations for filing, reviewing, and resolving chargebacks.
- **Embedded GraphiQL IDE:** Zero-setup interactive query and mutation playground.
- **Hierarchical RBAC:** Strict JWT authorization enforcing 4 role levels (`ADMIN` &rarr; `ANALYST` &rarr; `AUDITOR` &rarr; `USER`).
- **Native `pgx/v5` Concurrency:** High-performance PostgreSQL connection pool with prepared statements and 30-second statement timeouts.

---

## 2. File & Function Code Architecture

### `cmd/server/main.go` — Service Wiring & Lifecycle
- Boots the database connection pool (`pgxpool.New`), initializes repositories, wires the GraphQL schema, and launches the HTTP router on port 8083.

### `internal/graphql/` — Schema & Resolvers
- `schema.go`: Defines GraphQL types (`Dispute`, `User`, `Evidence`, `DisputeStatus`, `Role`).
- `resolvers.go`: Resolves queries (`disputes`, `dispute(id)`) and mutations (`createDispute`, `updateDisputeStatus`, `addEvidence`), checking the caller's JWT claims.

### `internal/auth/` — JWT & RBAC Engine
- `jwt.go`: Generates and validates HS256 JWT tokens.
- `rbac.go`: Enforces the role hierarchy ensuring that lower-tier roles cannot approve chargeback settlements or alter audit trails.

### `internal/api/router.go` & `web/` — HTTP Gateway (Port 8083)
- `GET /`: Interactive documentation and API portal.
- `GET /graphql`: Embedded GraphiQL IDE.
- `POST /graphql`: GraphQL execution endpoint.
- `GET /health` & `GET /ready`: Kubernetes health and readiness probes.

---

## 3. How to Run the Live Demo

### Starting the Server (Port 8083)
```powershell
.\recourse.exe
# or: go run ./cmd/server
```
Open your browser to: **http://127.0.0.1:8083** (or `/graphql` for GraphiQL)

### Demo Walkthrough Script
1. **Show the GraphiQL IDE:** Navigate to `http://127.0.0.1:8083/graphql`.
2. **Execute Sample Query:**
   ```graphql
   query GetAllDisputes {
     disputes {
       id
       amount
       currency
       reason
       status
       createdAt
     }
   }
   ```
3. **Execute Dispute Mutation:**
   ```graphql
   mutation FileChargeback {
     createDispute(input: {
       transactionId: "txn_987654321"
       amount: 450.00
       currency: "USD"
       reason: "FRAUDULENT_CARD_NOT_PRESENT"
     }) {
       id
       status
       amount
     }
   }
   ```
4. **Demonstrate Role Guard:** Show that attempting to resolve a dispute with a regular `USER` token is rejected with an HTTP 403 / GraphQL authorization error, while an `ANALYST` token succeeds.
