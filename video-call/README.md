# 🎥 Video Call App

Real-time peer-to-peer video calling using **WebRTC (UDP)** for media, **WebSocket** for signaling, and **JWT** for auth.

## Stack

| Layer | Tech |
|-------|------|
| Frontend | React 18 + TypeScript + Vite |
| Backend | Go 1.21 + gorilla/websocket + sqlite3 |
| Media | WebRTC (DTLS-SRTP over UDP) |
| Auth | JWT (HS256) + bcrypt |
| Infra | Docker Compose + nginx |
| Tunnel | ngrok |

---

## Quick Start (Local Dev)

```bash
# 1. Install deps
make install

# 2. Run backend + frontend in parallel
make dev

# 3. Open http://localhost:5173
```

## Docker

```bash
# Build & start all containers (detached)
make up

# Frontend → http://localhost:3000
# Backend  → http://localhost:8080
```

## Public URL via ngrok

> Requires [ngrok](https://ngrok.com/download) installed and authenticated.

```bash
# After `make dev` (Vite on 5173):
make ngrok

# After `make up` (nginx on 3000):
make ngrok-docker
```

ngrok will print a public HTTPS URL like `https://abc123.ngrok.io`.  
Open it on any device — share with anyone to test cross-device calls.

---

## API

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| POST | `/api/auth/register` | — | Create account |
| POST | `/api/auth/login` | — | Sign in |
| GET | `/api/auth/me` | Bearer | Current user |
| GET | `/api/users` | Bearer | List users |
| WS | `/ws?token=<jwt>` | JWT query | Signaling channel |

## How the call works

```
Alice ──call──▶ WebSocket Hub ──▶ Bob
Alice ◀──offer── (SDP via WS)
Bob  ──answer──▶ WS Hub ──▶ Alice
Both exchange ICE candidates (UDP ports)
       ┌─────────────────┐
       │  WebRTC P2P     │
       │  (UDP / SRTP)   │
       └─────────────────┘
```
