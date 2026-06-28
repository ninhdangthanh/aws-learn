# RAG Chatbot AI Backend - Project Plan

> Roadmap only. Architecture, schemas, API contracts, project structure, and design notes live in [docs/project-reference.md](./docs/project-reference.md).

## Implementation Roadmap

### Phase 1 - Project Foundation

> Goal: Project skeleton and local infrastructure are ready.

| # | Task | Details | Done |
|---|------|---------|------|
| 1.1 | Project scaffold | `go mod init`, folder structure, `cmd/api`, `cmd/worker`, `internal/*` | [x] |
| 1.2 | Makefile | `run-api`, `run-worker`, `test`, `migrate-up`, `docker-up` | [x] |
| 1.3 | Environment config | `.env.example`, Viper config loading | [x] |
| 1.4 | Docker Compose | PostgreSQL 16 + Redis 7 + Qdrant latest | [x] |
| 1.5 | App bootstrap | API starts and returns a simple health response | [x] |

Acceptance criteria:

- `docker-compose up -d` starts PostgreSQL, Redis, and Qdrant.
- API and worker binaries compile.
- Config loads from environment variables.

### Phase 2 - Database & Repositories

> Goal: Metadata persistence is ready before ingestion logic starts.

| # | Task | Details | Done |
|---|------|---------|------|
| 2.1 | DB migrations | `documents`, `chunks`, `chat_sessions`, `chat_messages` | [x] |
| 2.2 | GORM setup | models, DB connection, repository wiring | [x] |
| 2.3 | Document repository | Create, get, list, update status | [x] |
| 2.4 | Chunk repository | Bulk insert chunks, get chunks by document | [x] |
| 2.5 | Chat repository skeleton | Sessions and messages basic methods | [x] |

Acceptance criteria:

- `make migrate-up` creates all required tables.
- Repositories compile and can be tested with local PostgreSQL.
- Document status can be updated through repository methods.

### Phase 3 - Document Upload API

> Goal: Client can upload a PDF and track document status.

| # | Task | Details | Done |
|---|------|---------|------|
| 3.1 | Router + middleware | Gin router, recovery, request logging | [x] |
| 3.2 | Upload endpoint | `POST /api/v1/documents`, save file, create DB record | [x] |
| 3.3 | Status endpoint | `GET /api/v1/documents/:id` | [x] |
| 3.4 | File validation | PDF only for MVP, size limit, error response | [x] |
| 3.5 | Upload smoke test | Test with `curl -F "file=@..."` | [x] |

Acceptance criteria:

- Upload returns `202 Accepted` with document ID and `pending` status.
- Invalid files return structured JSON errors.
- Status endpoint returns document metadata.

### Phase 4 - Parse & Chunk Pipeline

> Goal: Uploaded PDF becomes citation-ready chunks in PostgreSQL.

| # | Task | Details | Done |
|---|------|---------|------|
| 4.1 | Parser interface | Common interface for PDF now, DOCX later | [x] |
| 4.2 | PDF parser | Extract text and page numbers | [x] |
| 4.3 | Chunker | 500-token chunks, 100-token overlap | [x] |
| 4.4 | Save chunks | Persist content, page, chunk index, token count | [x] |
| 4.5 | Unit tests | Parser/chunker behavior with small fixture | [x] |

Acceptance criteria:

- A test PDF is converted into ordered chunks.
- Each chunk has page number and chunk index.
- Document status can move from `pending` to `chunked`.

### Phase 5 - Async Workers

> Goal: Upload triggers background processing instead of blocking the API.

| # | Task | Details | Done |
|---|------|---------|------|
| 5.1 | Asynq setup | Client in API, server in worker binary | [x] |
| 5.2 | Parse job payload | Include document ID and file path | [x] |
| 5.3 | Enqueue parse job | Upload endpoint creates job | [x] |
| 5.4 | Parse worker | Parse file, chunk text, save chunks | [x] |
| 5.5 | Failure handling | Set `failed` status and `error_msg` | [x] |

Acceptance criteria:

- Upload returns quickly while worker processes in background.
- Status moves `pending -> parsing -> chunked`.
- Failed parsing is visible through status endpoint.
- Parse processing is idempotent: already-`chunked` documents are skipped, and retries replace prior chunks before re-inserting.

### Phase 6 - Embedding & Vector Indexing

> Goal: Chunks are embedded and stored in Qdrant.

| # | Task | Details | Done |
|---|------|---------|------|
| 6.1 | Embedding service | Reusable `text -> vector` OpenAI client | [x] |
| 6.2 | Qdrant collection init | Auto-create collection on startup | [x] |
| 6.3 | Vector repository | Upsert points, delete by document, search skeleton | [x] |
| 6.4 | Embed job | Enqueue after parse job succeeds | [x] |
| 6.5 | Embedding worker | Batch embed chunks and upsert vectors | [x] |
| 6.6 | Store vector IDs | Save Qdrant point IDs on chunk records | [x] |

Acceptance criteria:

- Status moves `chunked -> embedding -> ready`.
- Chunks appear in Qdrant with document/page payload.
- Vectors can be inspected in Qdrant dashboard.

### Phase 7 - Semantic Search

> Goal: User query retrieves relevant chunks.

| # | Task | Details | Done |
|---|------|---------|------|
| 7.1 | Query embedding | Embed the search query | [x] |
| 7.2 | Qdrant search | Similarity search with `top_k` and threshold | [x] |
| 7.3 | Search service | Map Qdrant results to API response | [x] |
| 7.4 | Search endpoint | `POST /api/v1/search` | [x] |
| 7.5 | Manual relevance test | Test with known document questions | [x] |

Acceptance criteria:

- Search returns chunks with score, filename, page, and text.
- `top_k` and `score_threshold` work.
- Search latency is logged.

### Phase 8 - RAG Chat Non-Streaming

> Goal: Search results become grounded LLM answers with citations.

| # | Task | Details | Done |
|---|------|---------|------|
| 8.1 | LLM service | OpenAI Chat Completions client | [x] |
| 8.2 | Prompt template | Context-only answer instruction + citation format | [x] |
| 8.3 | RAG orchestration | Embed query -> retrieve chunks -> build prompt -> call LLM | [x] |
| 8.4 | Chat endpoint | `POST /api/v1/chat` non-streaming | [x] |
| 8.5 | Citation response | Return document/page/snippet from retrieved chunks | [x] |
| 8.6 | Usage tracking | Token usage and latency in response | [x] |

Acceptance criteria:

- Chat returns an answer grounded in retrieved chunks.
- Citations reference correct document and page.
- If context is insufficient, answer says it cannot answer from the documents.

### Phase 9 - Chat Sessions & History

> Goal: Basic conversation state works without making the MVP heavy.

| # | Task | Details | Done |
|---|------|---------|------|
| 9.1 | Session creation | Create session when `session_id` is missing | [x] |
| 9.2 | Save messages | Persist user and assistant messages | [x] |
| 9.3 | Prompt history | Include recent messages in prompt | [x] |
| 9.4 | Session list endpoint | `GET /api/v1/chat/sessions` | [x] |
| 9.5 | Message list endpoint | `GET /api/v1/chat/sessions/:id/messages` | [x] |

Acceptance criteria:

- Chat response includes `session_id`.
- Follow-up questions can use recent history.
- Session/message endpoints return stored data.

### Phase 10 - SSE Streaming

> Goal: Chat response can stream tokens to the client.

| # | Task | Details | Done |
|---|------|---------|------|
| 10.1 | Streaming LLM client | Consume streaming response from LLM provider | [ ] |
| 10.2 | SSE handler | Stream token events from `POST /api/v1/chat` | [ ] |
| 10.3 | Final events | Emit citations, token usage, latency, done event | [ ] |
| 10.4 | Curl test | Verify with `curl -N` | [ ] |

Acceptance criteria:

- `stream=true` returns SSE token events.
- Final SSE response includes citations and usage.
- Non-streaming chat still works.

### Phase 11 - Polish & Demo Readiness

> Goal: The MVP is stable enough to demo and explain.

| # | Task | Details | Done |
|---|------|---------|------|
| 11.1 | Health endpoint | Check PostgreSQL, Redis, Qdrant | [ ] |
| 11.2 | List documents | `GET /api/v1/documents` with pagination/status filter | [ ] |
| 11.3 | Delete document | Remove document, chunks, and Qdrant vectors | [ ] |
| 11.4 | Error handling | Consistent error response shape | [ ] |
| 11.5 | Logging | Request ID, latency, service errors | [ ] |
| 11.6 | README update | Setup, run, upload, search, chat, stream commands | [ ] |
| 11.7 | End-to-end test | Manual flow from upload to cited answer | [ ] |

Acceptance criteria:

- Health endpoint reports all dependency statuses.
- Delete cleans PostgreSQL and Qdrant.
- README has tested commands.
- Full demo flow works end-to-end.

### Phase 12 - Frontend React & User Experience

> Goal: Build a minimal responsive SPA for upload, status, search, and chat.

| # | Task | Details | Done |
|---|------|---------|------|
| 12.1 | React scaffold | Minimal Vite React app with a few pages | [ ] |
| 12.2 | Upload page | File input + upload button, show status from API | [ ] |
| 12.3 | Status list | Simple list of documents and status badges | [ ] |
| 12.4 | Chat/search page | Ask question and show answer with citations | [ ] |
| 12.5 | Mobile-first UI | Basic responsive layout for small screens | [ ] |
| 12.6 | Deploy static build | Deploy as static files on EC2/S3/CloudFront | [ ] |

Acceptance criteria:

- Browser can upload PDF and see document status.
- Chat/search flows work from a single lightweight SPA.
- UI is usable on mobile and desktop.
- Deployment path is documented for static hosting.
