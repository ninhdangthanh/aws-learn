# 🤖 RAG Chatbot AI Backend

> An Internal Document QA System powered by Retrieval-Augmented Generation (RAG), built with Go.
>
> Upload PDF documents → Ask questions in natural language → Get accurate answers with citations.

#### next project: (ETL-AI-insight-service)

---

## What Is This?

A backend system that lets you upload internal documents (PDFs) and ask questions about them using natural language. The system finds the most relevant parts of your documents and uses an LLM to generate accurate answers **with citations** pointing back to the exact source and page.

```
User: "What is the refund policy for delivery orders?"

AI:   "According to the refund policy document, delivery order refunds
       are processed within 3 business days. The customer must provide
       the order number and reason for the refund."

       📄 Source: refund-policy.pdf, Page 12
```

**This is NOT a simple OpenAI API wrapper.** It's a complete backend system with:
- Document ingestion pipeline (parse → chunk → embed → index)
- Vector similarity search
- LLM orchestration with prompt engineering
- Background workers with job queues
- SSE streaming responses
- Citation extraction

---

## Architecture

```
┌─────────────┐
│   Client     │
│ (curl/Postman)│
└──────┬──────┘
       │ HTTP / SSE
       ▼
┌──────────────────────────────────┐
│         Go API Server (Gin)       │
│                                   │
│  ┌──────────┐  ┌────────┐  ┌──────────┐
│  │ Document  │  │ Search │  │   Chat   │
│  │ Handler   │  │Handler │  │ Handler  │
│  └─────┬────┘  └───┬────┘  └────┬─────┘
└────────┼────────────┼────────────┼──────┘
         │            │            │
    ┌────▼────┐  ┌────▼────┐  ┌───▼────┐
    │  Redis  │  │ Qdrant  │  │OpenAI  │
    │ (Queue) │  │(Vectors)│  │ (LLM)  │
    └────┬────┘  └─────────┘  └────────┘
         │
    ┌────▼──────────────────┐
    │   Background Workers   │
    │  ┌────────┐ ┌────────┐│
    │  │ Parse  │ │ Embed  ││
    │  │Worker  │ │Worker  ││
    │  └────────┘ └────────┘│
    └───────────────────────┘
         │
    ┌────▼─────┐
    │PostgreSQL│
    │(Metadata)│
    └──────────┘
```

### How It Works

**Document Upload Flow:**

1. Client uploads a PDF → API saves metadata to PostgreSQL
2. Parse Worker extracts text and splits into chunks (500 tokens, 100 overlap)
3. Embed Worker converts chunks to vectors via OpenAI Embedding API
4. Vectors are stored in Qdrant with document metadata

**Chat Flow:**

1. User asks a question → question is embedded to a vector
2. Qdrant finds the top-5 most similar chunks (semantic search)
3. Chunks are assembled into a prompt with citation instructions
4. LLM generates an answer grounded in the retrieved context
5. Response includes the answer + source citations

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| API Server | Go + Gin |
| Background Jobs | Asynq (Redis-based) |
| Vector Database | Qdrant |
| Embedding Model | OpenAI `text-embedding-3-small` |
| LLM | OpenAI GPT-4.1-mini |
| PDF Parsing | pdfcpu / unipdf |
| Database | PostgreSQL (metadata + chat history) |
| Queue / Cache | Redis |
| SQL Layer | GORM |
| Containerization | Docker + docker-compose |

---

## Getting Started

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- OpenAI API key

### 1. Clone & Setup

```bash
git clone https://github.com/ninhdangthanh/rag-chatbot.git
cd rag-chatbot

cp .env.example .env
# Edit .env → add your OPENAI_API_KEY
```

### 2. Start Infrastructure

```bash
docker-compose up -d
```

This starts:
- **PostgreSQL** on `localhost:5432`
- **Redis** on `localhost:6379`
- **Qdrant** on `localhost:6333` (dashboard: `http://localhost:6333/dashboard`)

### 3. Run Migrations

```bash
make migrate-up
```

### 4. Start the Services

```bash
# Terminal 1: API server
make run-api

# Terminal 2: Background workers
make run-worker
```

The API server starts on `http://localhost:8099`.

---

## API Usage

### Upload a Document

```bash
curl -X POST http://localhost:8099/api/v1/documents \
  -F "file=@your-document.pdf"
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "filename": "your-document.pdf",
  "status": "pending"
}
```

### Check Document Status

```bash
curl http://localhost:8099/api/v1/documents/550e8400-e29b-41d4-a716-446655440000
```

Wait for status to become `ready` (pending → parsing → chunked → embedding → ready).

### Search Documents

```bash
curl -X POST http://localhost:8099/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "What is the refund policy?",
    "top_k": 5
  }'
```

### Ask a Question

```bash
curl -X POST http://localhost:8099/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{
    "question": "What is the refund policy for delivery orders?",
    "stream": false
  }'
```

Response:
```json
{
  "answer": "According to the refund policy document...",
  "citations": [
    {
      "filename": "refund-policy.pdf",
      "page_number": 12,
      "text_snippet": "Refunds for delivery orders are processed..."
    }
  ],
  "token_usage": {
    "prompt_tokens": 850,
    "completion_tokens": 230
  },
  "latency_ms": 2300
}
```

### Streaming Response (SSE)

```bash
curl -N -X POST http://localhost:8099/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{
    "question": "What is the refund policy?",
    "stream": true
  }'
```

### Health Check

```bash
curl http://localhost:8099/api/v1/health
```

---

## Project Structure

```
rag-chatbot/
├── cmd/
│   ├── api/main.go              # API server entrypoint
│   └── worker/main.go           # Background worker entrypoint
├── internal/
│   ├── config/                  # Configuration (Viper)
│   ├── handler/                 # HTTP handlers
│   ├── service/                 # Business logic
│   ├── worker/                  # Asynq task handlers
│   ├── repository/              # Data access (PostgreSQL + Qdrant)
│   ├── model/                   # Domain structs
│   ├── parser/                  # PDF text extraction
│   ├── chunker/                 # Text chunking logic
│   ├── prompt/                  # LLM prompt templates
│   └── middleware/              # HTTP middleware
├── db/migrations/               # SQL migrations
├── docker-compose.yml
├── Makefile
└── .env.example
```

---

## Key Design Decisions

### Why Go instead of Python?

Most RAG tutorials use Python + LangChain. This project intentionally uses Go to demonstrate:
- The backend engineering challenges are language-agnostic
- Go excels at concurrent API servers, worker systems, and streaming
- Many production AI companies use Go for their backend infrastructure

### Why Asynq for background jobs?

Document ingestion is a multi-step pipeline (parse → chunk → embed) that must:
- Survive server restarts
- Support automatic retries on failure
- Provide visibility into job status

Goroutines alone can't provide these guarantees. Asynq (Redis-backed) gives us persistent, retryable, observable job processing.

### Why 500-token chunks with 100-token overlap?

- **500 tokens**: Large enough to contain meaningful context, small enough for precise retrieval
- **100-token overlap**: Prevents cutting sentences at chunk boundaries, ensures continuity across chunks
- This is a common starting point — the optimal size depends on your document types

### Why GORM for this demo?

GORM is a better fit for this repo's current pace:
- Fast CRUD scaffolding for early product phases
- Less generator/setup overhead while the API is still evolving
- Still paired with `golang-migrate`, so schema changes stay explicit

---

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/documents` | Upload a PDF document |
| `GET` | `/api/v1/documents` | List all documents |
| `GET` | `/api/v1/documents/:id` | Get document status |
| `DELETE` | `/api/v1/documents/:id` | Delete document + vectors |
| `POST` | `/api/v1/search` | Semantic search across documents |
| `POST` | `/api/v1/chat` | Ask a question (with optional streaming) |
| `GET` | `/api/v1/chat/sessions` | List chat sessions |
| `GET` | `/api/v1/chat/sessions/:id/messages` | Get session messages |
| `GET` | `/api/v1/health` | Health check |

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | API server port | `8099` |
| `OPENAI_API_KEY` | OpenAI API key | (required) |
| `OPENAI_EMBEDDING_MODEL` | Embedding model | `text-embedding-3-small` |
| `OPENAI_LLM_MODEL` | LLM model | `gpt-4.1-mini` |
| `POSTGRES_DSN` | PostgreSQL connection string | `postgres://user:pass@localhost:5432/ragchatbot?sslmode=disable` |
| `REDIS_ADDR` | Redis address | `localhost:6379` |
| `QDRANT_ADDR` | Qdrant gRPC address | `localhost:6334` |
| `CHUNK_SIZE` | Tokens per chunk | `500` |
| `CHUNK_OVERLAP` | Token overlap between chunks | `100` |
| `SEARCH_TOP_K` | Default number of results | `5` |

---

## Makefile Commands

```bash
make run-api        # Start API server
make run-worker     # Start background workers
make migrate-up     # Run database migrations
make migrate-down   # Rollback last migration
make docker-up      # Start infrastructure (PG + Redis + Qdrant)
make docker-down    # Stop infrastructure
make build          # Build both binaries
make test           # Run tests
make lint           # Run linter
```

---

## Further Reading

- [PROJECT_PLAN.md](./PROJECT_PLAN.md) — Focused MVP implementation roadmap
- [docs/project-reference.md](./docs/project-reference.md) — Architecture, schemas, API contracts, and design decisions
- [docs/idea.md](./docs/idea.md) — Original project idea and research notes

---

## License

MIT
