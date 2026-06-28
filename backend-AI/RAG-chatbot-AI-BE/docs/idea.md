RAG chatbot cho tài liệu nội bộ 1 tuần · Entry Upload PDF/docx → chunk → embed → lưu Qdrant → query → LLM trả lời có citation. Expose qua FastAPI. Stack: Python · FastAPI · Qdrant · OpenAI/Anthropic API · LangChain · Docker tôi là backend developer 3yoe, đang làm phần mềm FNB, retail truyền thống muốn switch qua hướng Backend developer for AI product, tôi có research project này, đây là một project? nếu đúng, thì có cần train AI không, và giúp tôi viết plan cho project này để tôi implement bằng Golang (keep project simple, chỉ để hiểu role backend of AI product,không cần làm level production cho các tính năng backend khác)

**Đúng, đây là một project backend AI rất hợp để transition từ backend truyền thống → AI product backend.**
Và quan trọng: **RAG chatbot không cần “train AI model” theo kiểu fine-tuning từ đầu.**

Bạn đang build một hệ thống:

> dữ liệu nội bộ → indexing/search → LLM reasoning → trả lời có citation

Đây là hướng rất thực tế trong công ty AI hiện nay.

---

# 1. Đây thực chất là project gì?

Đây là:

* AI Infra / AI Backend project
* Retrieval-Augmented Generation (RAG)
* Knowledge Base Chatbot
* Internal Document QA System

Kiểu use case rất phổ biến:

* chatbot cho SOP nội bộ
* hỏi policy công ty
* support docs
* technical docs
* legal docs
* HR docs
* retail operation docs
* F&B operation handbook

Ví dụ:

> “Quy trình refund cho đơn delivery là gì?”

Hệ thống sẽ:

1. tìm đoạn liên quan trong document
2. đưa context cho LLM
3. LLM trả lời
4. citation nguồn PDF/page

---

# 2. Có cần train AI không?

**Không bắt buộc.**

90% production RAG system hiện nay KHÔNG train model.

Bạn sẽ dùng:

| Thành phần      | Vai trò               |
| --------------- | --------------------- |
| Embedding model | convert text → vector |
| Vector DB       | semantic search       |
| LLM API         | generate answer       |
| Your backend    | orchestration         |

Flow:

```text
PDF
 → chunk text
 → embedding
 → save vectors to Qdrant

User question
 → embedding question
 → vector search
 → lấy top-k chunks
 → prompt LLM
 → answer + citations
```

---

# 3. “AI engineering” ở đây thực sự là gì?

Đây là phần nhiều backend engineer underestimate.

Phần khó KHÔNG phải model.

Phần khó là:

* ingestion pipeline
* chunking strategy
* metadata design
* retrieval quality
* prompt engineering
* hallucination reduction
* citation correctness
* caching
* concurrency
* observability
* cost optimization

Tức là:

> “AI product backend” thực ra cực kỳ gần backend engineering.

Nên background Go backend của bạn rất có lợi.

---

# 4. Có nên implement bằng Golang?

**Có. Rất nên.**

Hiện ecosystem AI Python mạnh hơn, nhưng:

* production backend
* high concurrency
* streaming
* API gateway
* orchestration
* ingestion workers

→ Go rất hợp.

Nhiều công ty dùng:

* Python cho ML/research
* Go cho AI infra/backend

---

# 5. Architecture nên làm bằng Go

## High-level architecture

```text
                ┌────────────┐
                │   Client   │
                └─────┬──────┘
                      │
                HTTP/WebSocket
                      │
             ┌────────▼────────┐
             │   Go API Server │
             │    (Gin/Fiber)  │
             └────────┬────────┘
                      │
      ┌───────────────┼────────────────┐
      │               │                │
      ▼               ▼                ▼

Document Service   Retrieval Service   Chat Service

      │               │                │
      ▼               ▼                ▼

PDF Parser      Qdrant Vector DB    OpenAI API

      │
      ▼

Chunk + Embed
```

---

# 6. Recommended Golang Stack

| Component       | Suggestion                    |
| --------------- | ----------------------------- |
| HTTP API        | Gin hoặc Fiber                |
| Background jobs | asynq / gocraft/work          |
| Vector DB       | Qdrant                        |
| Embedding API   | OpenAI text-embedding-3-small |
| LLM             | GPT-4.1 / Claude              |
| PDF parsing     | unidoc / pdfcpu               |
| DOCX parsing    | unioffice                     |
| ORM             | GORM                          |
| Queue           | Redis                         |
| Storage         | PostgreSQL                    |
| Container       | Docker                        |
| Observability   | Prometheus + Grafana          |

---

# 7. Project plan (backend-engineer style)

## Phase 1 — MVP Ingestion Pipeline

### Goal

Upload docs → searchable

### Tasks

#### 1. File upload API

```http
POST /documents
```

Store:

* filename
* size
* uploaded_by
* status

#### 2. Parse document

Support:

* PDF
* DOCX

Extract:

* plain text
* page numbers

#### 3. Chunking

Strategy:

```text
chunk_size = 200 tokens
overlap = 40
```

Metadata:

```json
{
  "document_id": "...",
  "page": 12,
  "chunk_index": 4
}
```

#### 4. Embedding worker

Flow:

```text
chunk
 → OpenAI embedding API
 → vector
 → save to Qdrant
```

#### 5. Qdrant collection design

Payload:

```json
{
  "document_id": "...",
  "page": 12,
  "text": "...",
  "title": "refund policy"
}
```

---

# 8. Phase 2 — Retrieval API

## Goal

Semantic search working

### Endpoint

```http
POST /search
```

Flow:

```text
query
 → embed query
 → Qdrant similarity search
 → top-k chunks
```

Return:

```json
{
  "chunks": [...]
}
```

---

# 9. Phase 3 — Chat API

## Goal

LLM answer with citations

### Endpoint

```http
POST /chat
```

Flow:

```text
User question
 → embed
 → retrieve chunks
 → build prompt
 → call LLM
 → return answer
```

Prompt example:

```text
Answer ONLY based on provided context.
If answer not found, say you don't know.

Context:
[1] ...
[2] ...

Question:
...
```

---

# 10. Phase 4 — Citation System

Very important for RAG quality.

Return:

```json
{
  "answer": "...",
  "citations": [
    {
      "document": "refund-policy.pdf",
      "page": 12
    }
  ]
}
```

---

# 11. Phase 5 — Production-level features

Đây là phần giúp project “senior-level”.

## Add:

### Hybrid search

Combine:

* BM25 keyword search
* vector search

### Re-ranking

Use reranker model:

```text
query + chunks
 → rerank relevance
```

### Conversation memory

Store:

* session
* previous questions

### Streaming response

SSE/WebSocket.

### Auth

JWT/OAuth.

### Multi-tenant

Per company knowledge base.

### Observability

Track:

* latency
* token usage
* hallucination rate
* retrieval score

---

# 12. Điều recruiter AI backend sẽ đánh giá ở project này

Họ không quan tâm bạn “train transformer”.

Họ sẽ nhìn:

## Backend engineering quality

* architecture
* async processing
* queue
* scaling
* retries
* idempotency
* observability
* API design

## AI product understanding

* chunking strategy
* retrieval quality
* prompt design
* hallucination mitigation
* vector DB usage
* citation handling

---

# 13. Nếu muốn project mạnh hơn nữa

Bạn có thể thêm:

## Advanced features

### A. Incremental indexing

Document changed → re-index only changed chunks.

### B. Access control

User chỉ search docs được phép xem.

### C. Table extraction

PDF tables → structured retrieval.

### D. Evaluation pipeline

Measure:

* precision@k
* recall
* groundedness

### E. Agent workflow

Ví dụ:

```text
LLM decides:
- search docs
- call tool
- summarize
```

---

# 14. Quan trọng: đừng biến project thành “wrapper OpenAI API”

Sai lầm phổ biến:

```text
Frontend
 → OpenAI API
```

Không có engineering depth.

Bạn nên focus:

* ingestion architecture
* retrieval pipeline
* worker system
* vector indexing
* scalable backend
* observability

Đó mới là thứ giúp bạn transition sang:

* AI Backend Engineer
* AI Infra Engineer
* LLM Platform Engineer
* Retrieval Engineer

---

# 15. Roadmap phù hợp cho bạn

Với background backend F&B/retail:

## Giai đoạn 1

Build RAG chatbot.

## Giai đoạn 2

Learn:

* embeddings
* vector DB
* prompt engineering
* evaluation

## Giai đoạn 3

Build:

* agent systems
* workflow orchestration
* AI memory systems
* tool calling

## Giai đoạn 4

Learn deeper infra:

* vLLM
* inference serving
* GPU basics
* model deployment

Bạn đang ở vị trí khá tốt để switch vì AI product rất cần người hiểu backend production chứ không chỉ ML theory.
