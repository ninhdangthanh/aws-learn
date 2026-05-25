# Usage — RAG Chatbot (Backend + Frontend)

This repository contains the Go backend for a RAG chatbot and a minimal Vite + React frontend in `/frontend`.

## Backend (what you already have)

Prerequisites:
- Go 1.20+
- Docker (for Postgres / Redis / Qdrant) or reachable services
- `make` (optional)

Quick start (local development):

1. Start infra (docker-compose):

```bash
# from repository root
docker-compose up -d
```

2. Create or copy `.env` from `.env.example` and set values (POSTGRES DSN, REDIS, QDRANT, OPENAI API KEY).

3. Run migrations (example):

```bash
make migrate-up
# or use golang-migrate directly if configured
```

4. Run the API server and worker in separate terminals:

```bash
# API server
go run ./cmd/api

# Worker
go run ./cmd/worker
```

By default the API listens on port `8080`. Endpoints:
- `POST /api/v1/documents` — upload file (multipart/form-data `file`)
- `GET /api/v1/documents` — list
- `GET /api/v1/documents/:id` — status
- `DELETE /api/v1/documents/:id` — delete
- `POST /api/v1/search` — semantic search
- `POST /api/v1/chat` — chat (RAG orchestration)
- `GET /api/v1/health` — service health

Notes:
- Ensure Qdrant, Postgres and Redis configs point to running services.
- Embeddings require an OpenAI API key (set in env).

## Frontend (Vite + React)

A minimal frontend is scaffolded under `/frontend`. It targets the backend endpoints and provides a simple UI to upload documents and ask questions.

Install and run:

```bash
cd frontend
npm install
npm run dev
```

The frontend defaults to `http://localhost:8080` for the API. To change, create a `.env` file under `/frontend` with:

```
VITE_API_BASE_URL=http://localhost:8080
```

Main features:
- Upload document (sends file to `POST /api/v1/documents`)
- Ask a question (sends JSON to `POST /api/v1/chat` and displays answer)

Files of interest:
- `/frontend/src/App.jsx` — simple UI
- `/frontend/src/api.js` — small wrapper for API calls
- `/frontend/index.html`, `/frontend/vite.config.js` — Vite setup

## Recommended workflow

1. Start infra with Docker Compose.
2. Start backend API and worker.
3. Start frontend (`npm run dev`) and open the URL shown by Vite.
4. Upload a PDF/DOCX and wait for the worker pipeline to process it (document status moves to `ready`).
5. Use the chat input to ask questions; the backend will perform embedding → retrieval → LLM answer.

## Troubleshooting

- If uploads return errors, check API logs and make sure Asynq + Redis are running.
- If embeddings fail, verify `OPENAI_API_KEY` env is set and correct.
- If Qdrant is unreachable, ensure `QDRANT_ADDR` in `.env` points to the Qdrant HTTP endpoint (e.g., `http://localhost:6333`).

---

If you want, I can:
- Add a small `Makefile` target to run frontend + backend together
- Wire CORS in the API to allow the frontend origin
- Implement a simple status indicator in the frontend to show document processing state

Which of those would you like next?