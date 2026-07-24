# LEARNING

Personal repo collecting everything from my backend self-study: theory notes, small runnable labs, and a few larger full-stack projects used for interview demos.

Every top-level directory is an **independent project/lab** with its own `go.mod`/`package.json` and README — clone it, `cd` in, and run. Nothing depends on anything else. Theory lives in [`notes/`](notes/readme.md); the rest is code.

---

## Notes and theory

| Directory | What's inside |
|---|---|
| [notes/](notes/readme.md) | Middle Backend interview notes: Golang, TypeScript, PostgreSQL, MongoDB, Redis, RabbitMQ, gRPC, Elasticsearch, security, JWT/session, EDA, system design. `readme.md` is the index plus a 3-week study plan |
| [GoF/](GoF/note.md) | GoF design patterns in Go, one runnable module per pattern: Strategy, Command, Chain of Responsibility, Decorator, Observer, State, Template Method, Composition |
| [game-livecode/](game-livecode/README.md) | Live-coding practice for board-game problems: Block Puzzle (Woodoku) and Tetris. Plain Go with tests, plus "retest" copies to redo from scratch; notes walk through the modelling steps |

---

## Pattern demos (Go, small, runnable)

Each demo deliberately isolates **one** concept — nothing is mixed in, so the code stays readable.

| Directory | What's inside |
|---|---|
| [go-circuit-breaker-demo/](go-circuit-breaker-demo/README.md) | Circuit breaker using `sony/gobreaker`: an order service calling a payment gateway, walking through all three states (closed/open/half-open) across 6 scenarios |
| [idempotency/](idempotency/README.md) | Idempotency lab covering the three places it usually shows up: HTTP APIs with side effects, queue consumers, and external providers. Go + PostgreSQL + Redis with `docker-compose` and migrations |
| [queueEdgeCases/](queueEdgeCases/readme.md) | RabbitMQ consumer edge cases: out-of-order messages, duplicate delivery, and safe retries via delay queue + DLQ |
| [rate-limit/](rate-limit/README.md) | Five rate-limiting algorithms, each in its own Go package: fixed window counter, sliding window log, sliding window counter, token bucket, leaky bucket |
| [asynq/](asynq/README.md) | Background jobs with [Asynq](https://github.com/hibiken/asynq) on Redis: `cmd/producer` enqueues tasks, a worker processes them |
| [testDbLockTableProd/](testDbLockTableProd/README.md) | PostgreSQL locking from a production angle: `ADD COLUMN`, `CREATE INDEX` vs `CONCURRENTLY`, long transactions, and a `watchlocks` command to observe real locks |

---

## Stack labs and full-stack projects

| Directory | What's inside |
|---|---|
| [elasticsearchStack/](elasticsearchStack/README.md) | ES 8.14 + Kibana (Docker) → Go/Gin backend → React/Vite search UI. Built in 7 phases: index/mapping, query DSL, aggregations, dashboards, DB→ES sync (dual-write vs outbox), and a real search feature (highlighting, facets, synonyms, multi-tenant filtering). See the [plan](elasticsearchStack/elasticsearch-implement-plan.md) and [deep-dive Q&A](elasticsearchStack/deep-dive-qa.md) |
| [kafka-project/](kafka-project/README.md) | Realtime commerce event platform for learning Kafka: Go backend + React frontend, full docker-compose stack. Includes `ARCHITECTURE.md`, `DEVELOPMENT_GUIDE.md`, and phase-by-phase tests |
| [multipartS3Upload/](multipartS3Upload/README.md) | Large-file uploads straight to S3 via presigned part URLs: React/Vite frontend, Go backend that only orchestrates (init → presign → complete/abort) and never touches the bytes. `docs/` holds the flow notes and S3 lifecycle/CORS config |
| [video-call/](video-call/README.md) | Peer-to-peer video calling: WebRTC (DTLS-SRTP over UDP) for media, WebSocket for signaling, JWT for auth. Go + gorilla/websocket + SQLite, React/TS frontend, deployed with Docker Compose + nginx |
| [go-template/](go-template/) | Reusable Go service boilerplate: api, config, database, migrations, redis, elastic, grpc, messaging, Dockerfile, Makefile, docker-compose |

---

## Authentication

| Directory | What's inside |
|---|---|
| [passkey-example/](passkey-example/README.md) | Passwordless auth with Passkeys/WebAuthn: register and sign in with biometrics, a device PIN, or a security key. Node backend (`server.js`) + frontend, with a [Vietnamese walkthrough](passkey-example/HUONG_DAN.md) |
| [MFA-example/](MFA-example/README.md) | Adding MFA on top of a normal password login, with two mechanisms: TOTP (Google Authenticator/Authy) and Passkeys as a second factor. Includes a [Vietnamese walkthrough](MFA-example/HUONG_DAN.md) |

---

## AWS

| Directory | What's inside |
|---|---|
| [aws-learning/](aws-learning/) | Five Node.js Lambda labs of increasing scope: receive a message, Lambda calling Lambda, S3 trigger, API hello, EventBridge on a schedule — plus `lambda-cicd` for automated deploys |
| [gateway-lab-notes-api/](gateway-lab-notes-api/README.md) | API Gateway lab: JWT/API keys, authorizers, usage plans with throttling/quota, request validation, CloudWatch logs. Currently only [PLAN.md](gateway-lab-notes-api/PLAN.md) and [DEPLOY_PLAN.md](gateway-lab-notes-api/DEPLOY_PLAN.md) — no code yet |

CI/CD for both lives in [`.github/workflows/`](.github/workflows) (`deploy-lambda-cicd.yml`, `video-call-deploy.yml`).

---

## AI backend

| Directory | What's inside |
|---|---|
| [backend-AI/](backend-AI/readme.md) | AI-agentic system concepts seen from a backend/distributed-systems angle, plus two sub-services |
| └ `RAG-chatbot-AI-BE/` | Go RAG chatbot backend: `cmd/api` + `cmd/worker`, PDF ingestion, vector search (Qdrant), PostgreSQL metadata, Redis for worker/cache, with a frontend and deploy config |
| └ `ETL-AI-insight-service/` | AI insight API over ETL data — still design-only, in [`idea.md`](backend-AI/ETL-AI-insight-service/idea.md) |

---

## Notes

* The root `docker-compose.yml` is just an nginx + PostgreSQL sandbox for quick tests, not infrastructure for any project. Each project ships its own compose file.
* Labs that use `docker-compose` (idempotency, queueEdgeCases, testDbLockTableProd, elasticsearchStack, kafka-project, video-call) need `docker compose up -d` before `go run`.
* Notes under `notes/` link to code with relative `../<project>` paths — keep the directory layout intact or those links break.
