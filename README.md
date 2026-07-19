# SaaS Backend Starter Kit

A production-shaped Go backend for multi-tenant SaaS products: JWT auth
with revocable sessions, organizations with role-based access control,
Stripe subscription billing, background jobs, and the operational
plumbing (health checks, metrics, structured logs, rate limiting) that
usually gets bolted on late and done badly.

Built to demonstrate the patterns I use on client backend projects —
see [`docs/architecture.md`](docs/architecture.md) for the reasoning
behind each one.

## Problem

Most SaaS MVPs need the same backend foundation — accounts, teams,
permissions, billing, a way to do slow work off the request path — and
most of it is undifferentiated. Client engagements go faster and safer
when that foundation is already solved correctly, so the actual project
work starts on day one instead of week three.

## What's built

- **Auth** — signup/login, short-lived JWT access tokens, revocable
  opaque refresh tokens with rotation, bcrypt password hashing (cost 12)
- **Multi-tenancy** — organizations, memberships, three-tier RBAC
  (owner/admin/member) enforced in one middleware
- **Billing** — Stripe Checkout, billing portal, webhook handling with
  hand-verified HMAC signatures (no SDK dependency for this)
- **Background jobs** — Redis-backed queue with retry + dead-letter
  handling, decoupling slow work (email) from the request/response cycle
- **Rate limiting** — Redis fixed-window, per-user or per-IP
- **Idempotency keys** — safe retries on POST/PATCH/PUT, the same
  pattern Stripe itself uses
- **Observability** — structured JSON logs (`log/slog`), `/healthz` +
  `/readyz`, a dependency-free `/metrics` endpoint in Prometheus text
  format
- **21 passing tests** covering auth, JWT lifecycle, webhook signature
  verification, and every middleware, run in CI on every push

## Stack

Go 1.22 · chi router · PostgreSQL (`database/sql`, no ORM) · Redis ·
Stripe · Docker · GitHub Actions

Deliberately minimal-dependency: no ORM, no logging framework, no job
queue library, no Stripe SDK — each of those is either standard library
or a ~100-line hand-rolled implementation. That's a design choice, not
an oversight; see the architecture doc for why.

## Running it locally

```bash
cp .env.example .env
docker compose up --build
```

This starts Postgres, Redis, runs migrations, and boots the API on
`localhost:8080`. Try it:

```bash
curl -X POST localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"correct-horse-battery-staple","name":"You"}'
```

Full endpoint reference: [`docs/openapi.yaml`](docs/openapi.yaml).

## Running the tests

```bash
make test          # go test ./... -race
make test-cover    # with coverage report
```

No Docker required for the test suite — Redis-dependent tests run
against an in-memory Redis (`miniredis`), not a live instance.

## Project layout

```
cmd/api/            entrypoint — wires config, DB, Redis, routes, worker
internal/
  auth/              signup, login, JWT, refresh token rotation
  tenant/             organizations, memberships, RBAC middleware
  billing/           Stripe checkout, portal, webhook verification
  middleware/         auth, rate limiting, idempotency, access logging
  worker/            Redis-backed background job queue
  httpserver/         router, health checks, metrics, graceful shutdown
  db/                Postgres + Redis connection setup
  config/            env-driven configuration with production guardrails
  logger/            structured logging (log/slog)
  response/          consistent JSON response envelope
migrations/          SQL schema migrations (golang-migrate compatible)
docs/                architecture notes + OpenAPI spec
```

## Deploying

The `Dockerfile` builds a static binary into a distroless image
(~15MB). It's ready to push to Fly.io, Railway, Render, or ECS as-is —
set the environment variables from `.env.example` on whichever platform
you use, point `DATABASE_URL` and `REDIS_URL` at managed instances, and
run the `migrate` step from `docker-compose.yml` once against
production.

## About

Built by [Divyesh Kakadiya](https://divyeshkakadiya.me) — Rust and Go
backend engineer focused on distributed systems and financial-grade
correctness. More work: [`rustmq`](https://github.com/Divyesh-k) (async
task queue with WAL + DLQ) and
[`transaction-service`](https://github.com/Divyesh-k) (ACID-safe
financial backend). Technical writing on
[Medium](https://medium.com/@divyeshkakadiya7531).
