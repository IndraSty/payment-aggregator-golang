# 💳 Payment Aggregator Middleware

<div align="center">

![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=for-the-badge&logo=go)
![Echo](https://img.shields.io/badge/Echo-v4-00ADD8?style=for-the-badge)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-316192?style=for-the-badge&logo=postgresql)
![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=for-the-badge&logo=redis)
![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)

A production-ready payment aggregator middleware that unifies **Midtrans**, **Xendit**, and **Stripe** into a single, clean API.

[Features](#-features) • [Architecture](#-architecture) • [API Docs](#-api-endpoints) • [Getting Started](#-getting-started) • [Security](#-security)

</div>

---

## 📌 Overview

Payment Aggregator Middleware eliminates the complexity of integrating multiple payment providers. Instead of maintaining separate integrations for each provider, clients hit **one unified API** — the system handles routing, webhook processing, double-charge prevention, and automatic reconciliation.

```
Client → Payment Aggregator → Midtrans  (IDR)
                            → Xendit    (IDR, fallback)
                            → Stripe    (USD, EUR)
```

---

## ✨ Features

| Feature | Description |
|---------|-------------|
| 🔀 **Smart Routing** | Auto-routes to the correct provider based on currency (IDR → Midtrans/Xendit, USD/EUR → Stripe) |
| 🛡️ **Double Charge Prevention** | Idempotency key via `X-Idempotency-Key` header with Redis TTL 24 hours |
| 🔌 **Provider Abstraction** | All providers implement the same interface — add new providers without touching existing code |
| 🪝 **Unified Webhooks** | Single handler for all three providers with signature verification |
| ⚡ **Circuit Breaker** | Per-provider circuit breaker with auto-fallback using sony/gobreaker |
| 🔄 **Auto Reconciliation** | Daily cron job comparing local state vs provider — flags discrepancies automatically |
| 📊 **Observability** | Prometheus metrics + structured logging with zerolog |
| 🔒 **Payment-Grade Security** | JWT auth, API key hashing, replay attack prevention, rate limiting |
| 📝 **Audit Trail** | Append-only audit log for every transaction status change |

---

## 🏗️ Architecture

### Service Architecture

How the system fits together end-to-end — from client request to payment provider and back.

```
                      ┌─────────────┐
                      │   Client    │
                      └──────┬──────┘
                             │ HTTPS Request
                             │ + X-Idempotency-Key
                             │ + Authorization: Bearer
                      ┌──────▼───────────────────────┐
                      │     Payment Aggregator        │
                      │                               │
                      │  ┌─────────────────────────┐  │
                      │  │     JWT Middleware       │  │
                      │  │     Rate Limiter         │  │
                      │  │     Idempotency Check    │  │
                      │  └────────────┬────────────┘  │
                      │               │                │
                      │  ┌────────────▼────────────┐  │
                      │  │     Provider Router      │  │
                      │  │  (currency-based routing)│  │
                      │  └──┬──────────┬─────────┬─┘  │
                      └─────┼──────────┼─────────┼────┘
                            │          │         │
                   IDR ─────┘    IDR ──┘   USD/EUR
                   (primary)  (fallback)   (only)
                            │          │         │
                   ┌────────▼───┐ ┌────▼────┐ ┌─▼──────┐
                   │  Midtrans  │ │ Xendit  │ │ Stripe │
                   │  Sandbox   │ │ Sandbox │ │  Test  │
                   └────────┬───┘ └────┬────┘ └─┬──────┘
                            │          │         │
                            └──────────┼─────────┘
                                       │ Webhook Callback
                      ┌────────────────▼──────────────┐
                      │        Payment Aggregator      │
                      │    (webhook handler layer)     │
                      └────────────────┬───────────────┘
                                       │
                      ┌────────────────▼───────────────┐
                      │          Storage Layer          │
                      │                                 │
                      │  ┌─────────────┐ ┌──────────┐  │
                      │  │ PostgreSQL  │ │  Redis   │  │
                      │  │ (Neon.tech) │ │(Upstash) │  │
                      │  └─────────────┘ └──────────┘  │
                      └─────────────────────────────────┘
```

### Code Architecture

This project follows **Clean Architecture** with 4 strict layers. Dependencies only flow inward — the domain layer knows nothing about the outside world.

```
┌─────────────────────────────────────────────────┐
│                   delivery/http                  │  ← HTTP handlers, middleware, router
├─────────────────────────────────────────────────┤
│                    usecase                       │  ← Business logic
├─────────────────────────────────────────────────┤
│              repository + provider               │  ← Data access + payment providers
├─────────────────────────────────────────────────┤
│                    domain                        │  ← Entities, interfaces, errors
└─────────────────────────────────────────────────┘
```

### Project Structure

```
payment-aggregator/
├── cmd/api/main.go                  # Entry point + dependency wiring
├── config/config.go                 # Viper-based configuration
├── internal/
│   ├── domain/                      # Entities, interfaces, errors (no imports)
│   ├── usecase/                     # Business logic
│   │   ├── auth_usecase.go
│   │   ├── charge_usecase.go
│   │   ├── webhook_usecase.go
│   │   └── reconcile_usecase.go
│   ├── repository/
│   │   ├── postgres/                # PostgreSQL implementations
│   │   └── redis/                   # Redis implementations
│   ├── delivery/http/
│   │   ├── handler/                 # HTTP handlers
│   │   ├── middleware/              # Auth, idempotency, rate limit, security
│   │   └── router.go
│   └── provider/
│       ├── midtrans/                # Midtrans client + mapper
│       ├── xendit/                  # Xendit client + mapper
│       └── stripe/                  # Stripe client + mapper
├── pkg/
│   ├── circuitbreaker/              # sony/gobreaker wrapper
│   ├── database/                    # PostgreSQL + Redis connection
│   ├── logger/                      # zerolog setup + masking helpers
│   ├── metrics/                     # Prometheus metrics definitions
│   ├── scheduler/                   # robfig/cron wrapper
│   └── validator/                   # Input validation
├── migrations/                      # SQL migration files (golang-migrate)
├── docs/                            # Swagger generated docs
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

---

## 🚀 Getting Started

### Prerequisites

- Go 1.26+
- [Neon.tech](https://neon.tech) account (free PostgreSQL)
- [Upstash](https://upstash.com) account (free Redis)
- Midtrans Sandbox account
- Xendit account
- Stripe account

### Installation

```bash
# 1. Clone the repository
git clone https://github.com/IndraSty/payment-aggregator-golang.git
cd payment-aggregator-golang

# 2. Install dependencies
go mod download

# 3. Copy environment file
cp .env.example .env

# 4. Fill in your credentials in .env
# (see Environment Variables section below)

# 5. Install development tools
make setup

# 6. Run database migrations
make migrate-up

# 7. Start the server
make run
```

### Environment Variables

```env
# Application
APP_NAME=payment-aggregator
APP_ENV=development
APP_PORT=8080

# Security
JWT_SECRET=your-super-secret-jwt-key-minimum-32-characters

# Database (Neon.tech)
DATABASE_URL=postgresql://user:password@host.neon.tech/dbname?sslmode=require

# Cache (Upstash Redis)
REDIS_URL=rediss://default:password@host.upstash.io:6379

# Midtrans Sandbox
MIDTRANS_SERVER_KEY=SB-Mid-server-xxxxxxxxxxxx
MIDTRANS_CLIENT_KEY=SB-Mid-client-xxxxxxxxxxxx
MIDTRANS_IS_PRODUCTION=false

# Xendit
XENDIT_SECRET_KEY=xnd_development_xxxxxxxxxxxx
XENDIT_CALLBACK_TOKEN=your-callback-token
XENDIT_IS_PRODUCTION=false

# Stripe
STRIPE_SECRET_KEY=sk_test_xxxxxxxxxxxx
STRIPE_WEBHOOK_SECRET=whsec_xxxxxxxxxxxx
STRIPE_IS_PRODUCTION=false
```

---

## 📡 API Endpoints

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/auth/register` | Register new user, returns API key |
| `POST` | `/api/v1/auth/login` | Login, returns JWT token pair |
| `POST` | `/api/v1/auth/refresh` | Refresh access token |

### Charges
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `POST` | `/api/v1/charges` | Create payment charge | JWT + Idempotency Key |
| `GET` | `/api/v1/charges` | List charges with pagination | JWT |
| `GET` | `/api/v1/charges/:id` | Get charge by ID | JWT |
| `POST` | `/api/v1/charges/:id/expire` | Manually expire a charge | JWT |

### Webhooks
| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/webhooks/midtrans` | Midtrans payment notification |
| `POST` | `/webhooks/xendit` | Xendit payment notification |
| `POST` | `/webhooks/stripe` | Stripe payment notification |

### Reconciliation
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `POST` | `/api/v1/reconciliation/run` | Trigger manual reconciliation | JWT |
| `GET` | `/api/v1/reconciliation/reports` | List reconciliation reports | JWT |
| `GET` | `/api/v1/reconciliation/reports/:id` | Get report by ID | JWT |

### System
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check (PostgreSQL + Redis) |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/swagger/*` | Swagger UI (development only) |

---

## 💡 Usage Example

### 1. Register and get API key

```bash
curl -X POST https://your-api.com/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "securepassword"}'
```

```json
{
  "success": true,
  "data": {
    "user": { "id": "...", "email": "user@example.com" },
    "api_key": "a3f8c2...",
    "message": "Store your API key securely. It will not be shown again."
  }
}
```

### 2. Login and get JWT token

```bash
curl -X POST https://your-api.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "securepassword"}'
```

```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGci...",
    "refresh_token": "eyJhbGci...",
    "expires_in": 900
  }
}
```

### 3. Create a charge (IDR → auto-routes to Midtrans)

```bash
curl -X POST https://your-api.com/api/v1/charges \
  -H "Authorization: Bearer eyJhbGci..." \
  -H "X-Idempotency-Key: unique-key-001" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 150000,
    "currency": "IDR",
    "payment_method": "bank_transfer",
    "customer_name": "John Doe",
    "customer_email": "john@example.com",
    "description": "Order #001"
  }'
```

```json
{
  "success": true,
  "data": {
    "id": "fa4a3882-692a-4ea8-99cd-313bafebdf75",
    "provider": "midtrans",
    "amount": 150000,
    "currency": "IDR",
    "status": "pending",
    "metadata": {
      "redirect_url": "https://app.sandbox.midtrans.com/snap/...",
      "token": "a33c731a-..."
    },
    "expired_at": "2024-01-02T10:00:00Z"
  }
}
```

### 4. Currency-based auto-routing

```bash
# IDR → Midtrans (primary) or Xendit (fallback if Midtrans circuit is open)
"currency": "IDR"

# USD → Stripe
"currency": "USD"

# EUR → Stripe
"currency": "EUR"
```

---

## 🔒 Security

This project implements payment-grade security across all layers:

- **JWT Authentication** — Access token expires in 15 minutes, refresh token in 7 days
- **API Key Hashing** — Stored as SHA-256 hash, plaintext never persisted
- **Webhook Signature Verification**
  - Midtrans: SHA-512(order_id + status_code + gross_amount + server_key)
  - Xendit: Static callback token header validation
  - Stripe: `webhook.ConstructEvent()` with timestamp validation
- **Replay Attack Prevention** — Webhooks older than 5 minutes are rejected, event IDs tracked
- **Rate Limiting** — Sliding window per IP (brute force) + per API key (abuse prevention)
- **SQL Injection Prevention** — Parameterized queries everywhere, zero raw string interpolation
- **Security Headers** — `X-Content-Type-Options`, `X-Frame-Options`, `Strict-Transport-Security`, `X-XSS-Protection`
- **Sensitive Data Masking** — Card numbers and secrets never appear in logs
- **Append-Only Audit Log** — Database-level rules prevent UPDATE/DELETE on audit table

---

## 🔄 Transaction State Machine

```
              ┌─────────┐
              │ pending │
              └────┬────┘
       ┌───────────┼───────────┐
       ▼           ▼           ▼
  ┌─────────┐ ┌────────┐ ┌─────────┐
  │ success │ │ failed │ │ expired │
  └─────────┘ └────────┘ └─────────┘
  (terminal)  (terminal) (terminal)
```

Once a transaction reaches a terminal state, no further transitions are allowed.

---

## ⚡ Circuit Breaker

Each payment provider has an independent circuit breaker:

```
Closed (healthy) → too many failures → Open (blocked)
Open → timeout expires → Half-Open (testing)
Half-Open → success → Closed
Half-Open → failure → Open
```

When a provider's circuit is **Open**, the router automatically skips it and tries the next available provider for the same currency.

---

## 🗄️ Database Schema

```sql
users                    -- API clients with hashed API keys
transactions             -- Payment records with state machine
transaction_audit_logs   -- Append-only status change history
webhook_events           -- Incoming webhooks for idempotency
reconciliation_reports   -- Daily reconciliation results
```

---

## 🛠️ Development

```bash
# Run with hot reload
make dev

# Run all tests
make test

# Run tests with coverage
make coverage

# Generate swagger docs
make swagger

# Create new migration
make migrate-create name=add_new_table

# Run migrations
make migrate-up

# Rollback last migration
make migrate-down

# Start local infrastructure (PostgreSQL + Redis)
make docker-up
```

---

## 🧪 Testing

```bash
# Run all tests
go test ./... -v

# Run with race detector
go test ./... -v -race

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Test coverage includes:
- ✅ Domain state machine logic
- ✅ Charge usecase (success, idempotency, invalid input, provider failure)
- ✅ Auth usecase (register, login, wrong password, user enumeration prevention)
- ✅ Currency routing configuration

---

## 📦 Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.23 |
| Framework | Echo v4 |
| Database | PostgreSQL (Neon.tech) |
| Cache | Redis (Upstash) |
| Migration | golang-migrate |
| Circuit Breaker | sony/gobreaker |
| Scheduler | robfig/cron |
| Auth | golang-jwt/jwt |
| Logging | rs/zerolog |
| Metrics | Prometheus |
| Docs | swaggo/swag |
| Testing | testify |

---

## 📄 License

This project is licensed under the MIT License.

---

<div align="center">
Built with ❤️ by <a href="https://github.com/IndraSty">IndraSty</a>
</div>