# Backend AI README

This repository captures the core concepts of AI-agentic systems for backend engineers and emphasizes how distributed reasoning extends traditional distributed systems.

## Key Areas

- AI-Agentic systems
- LLM-based agent frameworks
  - LangChain
  - AutoGen
  - Semantic Kernel
- Tool use and function calling
- RAG (Retrieval-Augmented Generation)
- Multi-agent orchestration

---

# 1. AI-Agentic System là gì?

AI-Agentic systems mở rộng kiến trúc backend truyền thống bằng cách thêm một lớp reasoning dựa trên LLM.

Thay vì:

- request → business logic → database → response

thì AI-Agentic system là:

- request → LLM reasoning → chọn tool/service → gọi API/DB/vector search → tổng hợp → phản hồi

## Agent là gì?

Một agent là một AI có khả năng:

- hiểu mục tiêu
- tự quyết định bước tiếp theo
- gọi tool/API
- nhớ context
- tự retry / tự phân rã task

Ví dụ:

> “Tạo báo cáo doanh thu tuần này rồi gửi email cho CEO”

Agent có thể tự thực hiện:

1. Query database
2. Generate summary
3. Convert thành PDF
4. Gửi email
5. Retry nếu mail fail

### Nó giống:

- một backend worker thông minh
- cộng thêm reasoning/planning bằng LLM

---

# 2. Multi-Agent Orchestration

Thay vì chỉ dùng 1 AI làm tất cả, ta chia nhiệm vụ cho nhiều agent chuyên trách.

| Agent            | Nhiệm vụ          |
| ---------------- | ----------------- |
| Router Agent     | phân loại yêu cầu |
| Billing Agent    | xử lý thanh toán  |
| Technical Agent  | debug kỹ thuật    |
| Escalation Agent | chuyển human      |

Flow:

```text
User
  ↓
Router Agent
  ├── Billing Agent
  ├── Technical Agent
  └── Human Escalation
```

### Backend dev sẽ thấy rất giống:

- microservice orchestration
- workflow engine
- event-driven architecture

### Khác biệt chính:

- decision making dùng LLM thay vì hardcode if/else

---

# 3. LLM Frameworks

## LangChain

LangChain là framework phổ biến nhất cho AI workflow.

Nó giống:

- Express/NestJS nhưng cho AI workflow

Cho phép:

- chain prompt
- memory
- tool calling
- agent
- RAG
- workflow

Ví dụ:

```ts
const agent = createAgent({
  llm,
  tools: [searchTool, dbTool]
});
```

Agent sẽ tự decide:

- khi nào search
- khi nào query db
- khi nào trả lời

### Concept quan trọng

- Chains → pipeline step-by-step
- Agents → autonomous decision maker
- Tools → function/API callable
- Memory → context/history
- Retriever → search knowledge

---

## AutoGen

AutoGen là framework của Microsoft, mạnh ở multi-agent collaboration.

Rất phù hợp cho:

- nhiều AI nói chuyện với nhau
- autonomous collaboration

Ví dụ:

- Planner Agent
- Coder Agent
- Reviewer Agent

Chu trình:

```text
Planner → Coder → Reviewer
              ↑
              └── feedback
```

Dùng nhiều cho:

- AI coding system
- autonomous workflow
- research automation

---

## Semantic Kernel

Semantic Kernel cũng của Microsoft, thiên về enterprise integration.

Phù hợp với:

- .NET ecosystem
- plugin architecture
- enterprise backend
- internal AI platform
- Copilot system

Core concepts:

- AI function
- planner
- memory
- connectors

---

# 4. Tool Use / Function Calling

Đây là phần quan trọng nhất của AI agent: LLM không thể tự truy cập internet hoặc database.

Nó cần “tool”.

Ví dụ tool:

```ts
getWeather(city)
searchUser(id)
createOrder(data)
```

LLM sẽ output:

```json
{
  "tool": "searchUser",
  "arguments": {
    "id": 123
  }
}
```

Backend system thực hiện:

1. parse JSON
2. execute function
3. trả result lại cho model

Đây là mô hình giống RPC / function dispatch.

---

# 5. RAG (Retrieval-Augmented Generation)

RAG giúp giảm hallucination và đảm bảo LLM trả lời dựa trên dữ liệu thực tế.

Nó làm:

- inject dữ liệu riêng/private
- search knowledge realtime

Flow:

```text
Question
   ↓
Embedding
   ↓
Vector Search
   ↓
Relevant Docs
   ↓
LLM Answer
```

Ví dụ:

> “Policy refund công ty là gì?”

Hệ thống sẽ:

1. Search document liên quan
2. Inject vào prompt
3. LLM trả lời dựa trên doc

---

# 6. Embedding & Vector Database

## Embedding

Embedding biến text thành vector số để so sánh ý nghĩa.

Ví dụ:

```text
"I love cats"
↓
[0.123, -0.882, ...]
```

Mục tiêu:

- semantic similarity

## Vector DB

Vector DB lưu vector để truy vấn gần đúng.

Popular:

- Pinecone
- Weaviate
- Qdrant
- Milvus

Backend dev có thể hiểu nó như:

- Elasticsearch cho semantic meaning

Thay vì:

```sql
WHERE title LIKE '%refund%'
```

thì:

```text
find semantically similar documents
```

---

# 7. Orchestration

Đây là phần core của backend engineering cho AI system.

Một AI workflow thường có:

```text
API Gateway
   ↓
Agent Runtime
   ↓
Planner
   ↓
Tool Executor
   ↓
Memory / Vector DB
   ↓
LLM Provider
```

Các concern quen thuộc:

- retry
- timeout
- rate limit
- caching
- observability
- tracing
- queue
- concurrency
- token cost
- latency

AI system engineering hiện nay là:

> distributed systems + probabilistic reasoning

---

# 8. So sánh với backend truyền thống

| Backend truyền thống  | AI Agent System     |
| --------------------- | ------------------- |
| deterministic         | probabilistic       |
| if/else               | reasoning           |
| fixed workflow        | dynamic workflow    |
| REST call             | tool call           |
| DB query              | semantic retrieval  |
| service orchestration | agent orchestration |
| cron/job              | autonomous planning |

---

# 9. Một kiến trúc AI backend thực tế

Ví dụ AI customer support:

```text
Frontend
   ↓
API Gateway
   ↓
Conversation Service
   ↓
Agent Orchestrator
   ├── Retrieval Service
   ├── Order Service
   ├── Billing Service
   ├── CRM Tool
   └── Email Tool
   ↓
LLM
```

Backend tech stack thường dùng:

- Kafka/RabbitMQ
- Redis
- PostgreSQL
- Vector DB
- Kubernetes
- Temporal
- LangChain/LangGraph
- OpenAI/Claude/Gemini

---

# 10. Điều backend dev cần học đầu tiên

Thứ tự hợp lý:

1. Prompt engineering cơ bản
2. Function calling
3. RAG
4. Embedding/vector search
5. Agent workflow
6. Multi-agent orchestration
7. AI observability/evaluation

---

# 11. Quan trọng nhất: mindset shift

Backend truyền thống:

> code quyết định logic

AI system:

> model quyết định logic, code chỉ guardrail + orchestration

Đó là thay đổi lớn nhất.

---

## Tóm tắt

AI-Agentic systems với backend developer nghĩa là xây dựng hệ thống:

- vẫn giữ các nguyên tắc distributed system
- thêm layer reasoning, decision making và tool orchestration
- dùng LLM làm “brain”, nhưng backend làm “nerves” và “muscles”
