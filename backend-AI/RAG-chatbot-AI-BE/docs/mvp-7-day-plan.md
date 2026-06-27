# Kế Hoạch RAG MVP - 7 Ngày

## Mục tiêu

Trong 7 ngày, mỗi ngày 1.5-2 giờ, hoàn thành một RAG chatbot backend bằng Go có thể demo được:

```text
Upload PDF
-> lưu metadata Postgres
-> enqueue Redis/Asynq
-> worker parse + chunk
-> embed + lưu Qdrant
-> POST /search
-> POST /chat trả answer + citation
```

Đây là MVP để hiểu vai trò backend trong AI product, không phải production-ready RAG system.

## Scope

### Có làm

- Go, Gin, GORM, PostgreSQL, Redis/Asynq, Qdrant và OpenAI API.
- Upload PDF text-based.
- Xử lý bất đồng bộ: upload trả nhanh, worker xử lý ingestion.
- Semantic search và chat non-streaming có citation.
- Test cho chunker và 1-2 service quan trọng.

### Không làm

- Authentication/authorization, multi-tenant, frontend đẹp.
- PDF scan/OCR, DOCX, chat history, SSE streaming.
- Rate limit, Prometheus, tracing, microservice, Kubernetes.
- Tối ưu chi phí, đánh giá retrieval chuyên sâu hoặc production hardening.

## Kế hoạch theo ngày

| Ngày | Việc làm | Definition of done |
|---|---|---|
| 1 | Tạo Go module, cấu trúc `cmd/api`, `cmd/worker`, config; Docker Compose cho Postgres, Redis, Qdrant; Gin health endpoint | API và 3 dependency chạy được trên local |
| 2 | GORM migration `documents`, `chunks`; model và repository tối thiểu | Tạo/list document metadata từ Postgres |
| 3 | `POST /api/v1/documents`: validate PDF/size, lưu file local và record trạng thái `pending` | Upload trả `202 Accepted` cùng document ID |
| 4 | Thêm Asynq client/server; upload enqueue job; worker cập nhật `pending -> processing -> failed` | API không block khi worker chạy |
| 5 | PDF parser MVP, chunker có `page`, `chunk_index`, `content`; lưu chunks vào Postgres | Một PDF thành danh sách chunks đúng thứ tự |
| 6 | Embedding client, khởi tạo Qdrant collection, batch upsert vectors có payload citation; status thành `ready` | Vectors tìm thấy được trong Qdrant |
| 7 | `POST /api/v1/search` và `POST /api/v1/chat`; ghép context, gọi LLM, trả citation; cập nhật README | Demo được một PDF và tối thiểu 5 câu hỏi |

## Data model tối thiểu

```text
documents
- id
- filename
- file_path
- status: pending | processing | ready | failed
- error_message
- created_at

chunks
- id
- document_id
- page
- chunk_index
- content
- vector_id
```

Qdrant payload:

```json
{
  "document_id": "...",
  "chunk_id": "...",
  "filename": "policy.pdf",
  "page": 3,
  "content": "..."
}
```

## API tối thiểu

| Method | Endpoint | Mục đích |
|---|---|---|
| `GET` | `/api/v1/health` | Kiểm tra API còn sống |
| `POST` | `/api/v1/documents` | Upload PDF, tạo ingestion job |
| `GET` | `/api/v1/documents/:id` | Xem trạng thái xử lý |
| `POST` | `/api/v1/search` | Tìm chunks theo semantic search |
| `POST` | `/api/v1/chat` | Hỏi đáp RAG, trả lời kèm citation |

## Quy tắc khi bị chậm tiến độ

1. Chỉ hỗ trợ một file PDF text-based.
2. Dùng OpenAI SDK/API trực tiếp, không thêm LangChain.
3. Chunk theo ký tự/word, không cần tokenizer chính xác.
4. Làm response không streaming trước.
5. Nếu Qdrant hoặc LLM làm chậm ngày 6-7, demo đến semantic search trước rồi thêm chat khi có thời gian.

## Kịch bản demo

1. Khởi động Postgres, Redis, Qdrant, API và worker.
2. Upload một PDF có nội dung dễ kiểm chứng.
3. Gọi status đến khi document thành `ready`.
4. Search một câu hỏi và kiểm tra chunk/page trả về.
5. Chat cùng câu hỏi; kiểm tra answer chỉ dựa trên context và có citation file/page.

## Cách kể khi phỏng vấn

> “Em làm RAG backend bằng Go: API nhận document, Redis-backed worker xử lý ingestion để request upload không block, metadata/chunk nằm ở Postgres, embedding nằm ở Qdrant. Khi chat, em embed query, retrieve top-k chunks, chỉ đưa context này vào LLM và trả citation. Bản MVP ưu tiên status và failure handling của ingestion; em chưa tách microservice vì complexity chưa đáng.”
