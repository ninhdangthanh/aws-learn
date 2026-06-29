# Luồng xử lý và khái niệm

Ghi chú này tóm tắt các luồng RAG chính và vai trò của từng thành phần trong project.

## Thành phần chính

- **API server**: nhận request upload, search và chat.
- **PostgreSQL**: lưu metadata tài liệu, chunks, chat sessions và chat messages.
- **Redis / Asynq**: làm hàng đợi cho các background jobs như parse và embed tài liệu.
- **Worker**: xử lý các job parse và embed bất đồng bộ.
- **OpenAI Embeddings**: chuyển document chunks và câu hỏi người dùng thành vectors.
- **Qdrant**: lưu vectors và thực hiện semantic similarity search.
- **OpenAI Chat Model**: sinh câu trả lời cuối cùng dựa trên context đã retrieval.

## Luồng upload / ingestion

```text
Người dùng upload PDF
-> API validate và lưu PDF xuống disk
-> API tạo metadata tài liệu trong PostgreSQL với status=pending
-> API đẩy parse task vào Redis/Asynq
-> Worker parse PDF thành các trang
-> Worker chia text từng trang thành chunks
-> Worker lưu chunks vào PostgreSQL
-> Worker cập nhật document status=chunked
-> Worker đẩy embed task
-> Worker load chunks từ PostgreSQL
-> Worker gọi OpenAI Embeddings cho chunk text
-> Worker xóa vectors cũ của document trong Qdrant nếu có
-> Worker upsert vectors và payload vào Qdrant
-> Worker cập nhật chunk qdrant_id trong PostgreSQL
-> Worker cập nhật document status=ready
```

Chi tiết quan trọng: PostgreSQL giữ dữ liệu nghiệp vụ bền vững và records của chunks. Qdrant giữ searchable vector index và payload như `document_id`, `chunk_id`, `filename`, `page_number`, `chunk_index`, và `text`.

## Luồng search

```text
Người dùng gửi search query
-> API trim và validate query
-> Service gọi OpenAI Embeddings cho query text
-> Service query Qdrant bằng query vector
-> Qdrant trả về topK chunks tương tự nhất kèm score và payload
-> API trả matched chunks về client
```

Search endpoint chỉ lấy các chunks liên quan. Endpoint này không gọi chat model để sinh câu trả lời cuối cùng.

## Luồng chat

```text
Người dùng đặt câu hỏi
-> API tìm hoặc tạo chat session
-> Service load recent chat history từ PostgreSQL
-> Service gọi Search flow bằng câu hỏi
-> Search flow embed câu hỏi và retrieval các chunks liên quan từ Qdrant
-> Service dựng prompt gồm:
   - system instructions
   - câu hỏi của người dùng
   - lịch sử hội thoại gần đây
   - retrieved chunk text làm context
-> Service gọi OpenAI Chat Model
-> OpenAI trả về grounded answer
-> Service dựng citations từ retrieved chunks
-> Service lưu user message và assistant message vào PostgreSQL
-> API trả answer, citations, session_id, token usage và latency
```

Điểm cần làm rõ: chat model không nhận raw vectors. Vectors chỉ được dùng để tìm chunks liên quan trong Qdrant. Chat model nhận **text** của các chunks đã retrieval làm context.

## Vòng đời trạng thái tài liệu

```text
pending
-> parsing
-> chunked
-> embedding
-> ready
```

Nếu parse, embed, ghi database hoặc upsert vector thất bại, document được đánh dấu:

```text
failed
```

## Mô hình tư duy

RAG có hai pha tách biệt:

1. **Indexing phase**: chuyển tài liệu thành searchable chunks và vectors.
2. **Retrieval + generation phase**: chuyển câu hỏi người dùng thành vector, retrieval các chunks liên quan, rồi yêu cầu LLM trả lời chỉ dựa trên các chunks đó.

Tóm tắt ngắn:

```text
Documents -> chunks -> vectors -> Qdrant
Question -> vector -> relevant chunks -> LLM answer with citations
```

## Khái niệm chính

### Score Threshold

`score_threshold` là điểm similarity tối thiểu để một kết quả từ Qdrant được chấp nhận.

Ví dụ:

```text
chunk A score=0.86
chunk B score=0.72
chunk C score=0.31
```

Nếu `score_threshold=0.70`, chỉ chunk A và chunk B được trả về. Chunk C bị bỏ qua vì có khả năng không đủ liên quan.

Giá trị này hữu ích khi muốn tránh đưa context yếu hoặc không liên quan vào chat model.

### Citations

`citations` là nguồn tham chiếu dùng để hỗ trợ câu trả lời cuối cùng.

Trong project này, citations được dựng từ kết quả retrieval của Qdrant. Mỗi citation gồm các field như:

```text
chunk_id
document_id
filename
page_number
chunk_index
text_snippet
score
```

Hàm `buildCitations(results)` chuyển search results thành citation objects. Citations giúp người dùng kiểm chứng câu trả lời đến từ đâu.

### Cited Answer

Cited answer là câu trả lời có kèm nguồn tham chiếu.

Thay vì:

```text
Học sinh cần hiểu vai trò của công nghệ trong đời sống và sản xuất.
```

Assistant nên trả lời:

```text
Học sinh cần hiểu vai trò của công nghệ trong đời sống và sản xuất [cong-nghe-10-topic.pdf, trang 3].
```

Điều này quan trọng vì câu trả lời RAG nên được grounded trong tài liệu đã upload, không chỉ sinh từ kiến thức chung của model.

### Embedding

Embedding là quá trình chuyển text thành numeric vector.

Ví dụ:

```text
"Machine learning là một lĩnh vực của AI"
-> OpenAI Embedding API
-> [0.012, -0.034, 0.88, ...]
```

Trong project này:

```text
document chunk text -> OpenAI Embeddings -> chunk vector
user query text -> OpenAI Embeddings -> query vector
```

Qdrant không tự tạo embeddings. Qdrant lưu vectors và search trên vectors đó. OpenAI là bên tạo vectors.

### Vector Search

Vector search tìm text theo ý nghĩa, không chỉ theo keyword chính xác.

Trong project này, Qdrant dùng cosine similarity. Ý tưởng là:

```text
ý nghĩa giống nhau -> vectors gần nhau
ý nghĩa khác nhau -> vectors xa nhau
```

Khi người dùng hỏi, câu hỏi được embed thành vector. Qdrant so sánh query vector đó với chunk vectors đã lưu và trả về các chunks gần nhất.

### Chunking

Chunking là chia tài liệu thành các đoạn text nhỏ hơn trước khi embedding.

Cần chunk vì:

- Tài liệu đầy đủ thường quá lớn để embed hoặc đưa vào chat model hiệu quả.
- Chunks nhỏ giúp search chính xác hơn.
- Chat model chỉ cần các phần liên quan nhất, không cần toàn bộ tài liệu.

### Overlap

Overlap nghĩa là mỗi chunk chia sẻ một phần text với chunk trước hoặc chunk sau.

Ví dụ:

```text
chunk 1: tokens 1-200
chunk 2: tokens 161-360
chunk 3: tokens 321-520
```

Ở đây overlap là 40 tokens.

Overlap giúp tránh mất ngữ nghĩa ở ranh giới chunk. Nếu một ý quan trọng bắt đầu ở cuối chunk 1 và tiếp tục sang chunk 2, overlap giúp cả hai chunk giữ đủ context.

### TopK

`topK` kiểm soát số lượng chunks liên quan được trả về từ vector search.

Ví dụ:

```text
topK=5
```

nghĩa là Qdrant trả về 5 chunks gần nhất.

`topK` cao hơn có thể cho model nhiều context hơn, nhưng cũng có thể thêm nhiễu và tăng token cost.

### Grounding

Grounding nghĩa là ép model trả lời dựa trên document context đã retrieval thay vì dùng kiến thức chung.

Trong project này, grounding xảy ra qua:

- semantic search retrieval các chunks liên quan.
- system prompt yêu cầu "Answer using only the provided context".
- citations trỏ ngược về retrieved chunks.

### Retrieval

Retrieval là bước tìm các chunks liên quan từ vector database.

```text
question -> embedding -> Qdrant search -> relevant chunks
```

Chất lượng retrieval là một trong các phần quan trọng nhất của RAG. Nếu retrieval trả về context kém, câu trả lời cuối cùng thường cũng kém.

### Generation

Generation là bước chat model viết câu trả lời cuối cùng.

```text
question + recent history + retrieved chunks -> OpenAI Chat Model -> final answer
```

Chat model nên tổng hợp retrieved text thành câu trả lời hữu ích và kèm citations.

### Hallucination

Hallucination nghĩa là model nói điều không được hỗ trợ bởi nguồn hoặc sai sự thật.

RAG giảm hallucination bằng cách cung cấp document context đã retrieval và yêu cầu model chỉ trả lời dựa trên context đó. Tuy nhiên RAG không loại bỏ hallucination hoàn toàn, nên cited answers và source snippets vẫn rất quan trọng.

## Các luồng hỗ trợ

Các luồng RAG chính là upload, search và chat. Những luồng hỗ trợ dưới đây hữu ích khi debug hoặc vận hành project.

### Document Status Flow

```text
pending
-> parsing
-> chunked
-> embedding
-> ready
```

Nếu có lỗi trong quá trình parsing, embedding, ghi database hoặc vector upsert, document chuyển thành:

```text
failed
```

### Document List / Status / Delete Flow

```text
liệt kê documents
-> kiểm tra document status
-> xóa document khi cần
-> xóa vectors liên quan trong Qdrant
-> xóa file đã upload khỏi disk nếu còn tồn tại
```

Luồng này chủ yếu phục vụ quản lý UI và cleanup. Nó cũng hữu ích khi cần kiểm tra vì sao một tài liệu đã upload nhưng chưa search được.

### Chat Session Flow

```text
tạo hoặc lấy chat session
-> lưu user message
-> sinh assistant answer
-> lưu assistant message kèm citations và token usage
-> liệt kê sessions và messages cũ
```

Chat sessions cho phép app giữ lịch sử hội thoại gần đây và tái sử dụng lịch sử đó trong các prompt sau.

### Streaming Chat Flow

```text
chạy retrieval và generation giống luồng chat thường
-> trả tokens dần dần qua SSE
-> gửi citations
-> gửi done event
-> lưu lượt hội thoại cuối cùng
```

Streaming cải thiện trải nghiệm người dùng vì người dùng thấy câu trả lời được sinh dần thay vì phải chờ toàn bộ response hoàn thành.

### Health Flow

```text
kiểm tra API health
-> kiểm tra kết nối PostgreSQL
-> kiểm tra kết nối Qdrant
-> trả về health status
```

Health checks hữu ích cho deployment, uptime monitoring và debug nhanh sau khi Docker containers khởi động.

## Các khái niệm vẫn nên học thêm

Các câu hỏi ở trên bao phủ nền tảng cần thiết để hiểu project này. Sau đó, các khái niệm hữu ích tiếp theo là:

- **Recall vs precision**: recall là tìm đủ chunks liên quan; precision là tránh chunks không liên quan.
- **Chunk size tuning**: chunks nhỏ cải thiện precision, chunks lớn giữ được nhiều context hơn.
- **Embedding model choice**: model embedding khác nhau ảnh hưởng search quality, vector size, latency và cost.
- **Reranking**: sau vector search, reranker có thể sắp xếp lại kết quả để tăng relevance.
- **Hybrid search**: kết hợp vector search với keyword search để tốt hơn với exact terms, IDs, names và rare words.
- **Metadata filtering**: chỉ search trong một document, user workspace, category, date range hoặc permission scope.
- **Prompt construction**: quyết định cách sắp xếp retrieved chunks, history và instructions trước khi gọi chat model.
- **Context window and token budget**: kiểm soát lượng retrieved text có thể đưa vào final prompt.
- **Evaluation**: kiểm tra retrieval có tìm đúng chunks không và câu trả lời có trung thành với nguồn không.
- **Observability**: log latency, topK, scores, token usage, failed jobs và retrieval quality.

Với project này, nền tảng bắt buộc cần nắm là:

```text
chunking
embedding
vector search
topK
score_threshold
retrieval
grounded generation
citations
```
