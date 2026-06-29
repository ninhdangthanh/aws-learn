# Flow and Concept

This note summarizes the main RAG flows and the role of each component in this project.

## Core Components

- **API server**: receives upload, search, and chat requests.
- **PostgreSQL**: stores document metadata, chunks, chat sessions, and chat messages.
- **Redis / Asynq**: queues background jobs for parsing and embedding documents.
- **Worker**: processes parse and embed jobs asynchronously.
- **OpenAI Embeddings**: converts document chunks and user queries into vectors.
- **Qdrant**: stores vectors and performs semantic similarity search.
- **OpenAI Chat Model**: generates the final grounded answer from retrieved text context.

## Upload / Ingestion Flow

```text
User uploads PDF
-> API validates and saves PDF to disk
-> API creates document metadata in PostgreSQL with status=pending
-> API enqueues parse task to Redis/Asynq
-> Worker parses PDF into pages
-> Worker chunks page text
-> Worker saves chunks into PostgreSQL
-> Worker updates document status=chunked
-> Worker enqueues embed task
-> Worker loads chunks from PostgreSQL
-> Worker calls OpenAI Embeddings for chunk text
-> Worker deletes old vectors for the document in Qdrant if any
-> Worker upserts vectors and payload into Qdrant
-> Worker updates chunk qdrant_id in PostgreSQL
-> Worker updates document status=ready
```

Important detail: PostgreSQL keeps the durable business data and chunk records. Qdrant keeps the searchable vector index plus payload such as `document_id`, `chunk_id`, `filename`, `page_number`, `chunk_index`, and `text`.

## Search Flow

```text
User sends search query
-> API trims and validates query
-> Service calls OpenAI Embeddings for the query text
-> Service queries Qdrant with the query vector
-> Qdrant returns topK similar chunks with score and payload
-> API returns matched chunks to the client
```

The search endpoint only retrieves relevant chunks. It does not call the chat model to generate a final answer.

## Chat Flow

```text
User asks a question
-> API resolves or creates chat session
-> Service loads recent chat history from PostgreSQL
-> Service calls Search flow using the question
-> Search flow embeds the question and retrieves relevant chunks from Qdrant
-> Service builds a prompt with:
   - system instructions
   - user question
   - recent conversation history
   - retrieved chunk text as context
-> Service calls OpenAI Chat Model
-> OpenAI returns grounded answer
-> Service builds citations from retrieved chunks
-> Service saves user and assistant messages into PostgreSQL
-> API returns answer, citations, session_id, token usage, and latency
```

Key clarification: the chat model does not receive raw vectors. Vectors are only used to find relevant chunks in Qdrant. The chat model receives the retrieved chunk **text** as context.

## Status Lifecycle

```text
pending
-> parsing
-> chunked
-> embedding
-> ready
```

If parsing, embedding, database writes, or vector upsert fails, the document is marked as:

```text
failed
```

## Mental Model

RAG has two separate phases:

1. **Indexing phase**: turn documents into searchable chunks and vectors.
2. **Retrieval + generation phase**: turn a user question into a vector, retrieve relevant chunks, then ask the LLM to answer only from those chunks.

In short:

```text
Documents -> chunks -> vectors -> Qdrant
Question -> vector -> relevant chunks -> LLM answer with citations
```

## Key Concepts

### Score Threshold

`score_threshold` is the minimum similarity score required for a Qdrant result to be accepted.

Example:

```text
chunk A score=0.86
chunk B score=0.72
chunk C score=0.31
```

If `score_threshold=0.70`, only chunk A and chunk B are returned. Chunk C is ignored because it is probably not relevant enough.

This is useful when we want to avoid giving weak or unrelated context to the chat model.

### Citations

`citations` are the sources used to support the final answer.

In this project, citations are built from retrieved Qdrant results. Each citation includes fields such as:

```text
chunk_id
document_id
filename
page_number
chunk_index
text_snippet
score
```

The function `buildCitations(results)` converts search results into citation objects. Citations help users verify where the answer came from.

### Cited Answer

A cited answer is an answer with source references.

Instead of:

```text
Students need to understand the role of technology in life and production.
```

The assistant should answer:

```text
Students need to understand the role of technology in life and production [cong-nghe-10-topic.pdf, page 3].
```

This is important because a RAG answer should be grounded in uploaded documents, not just generated from the model's general knowledge.

### Embedding

Embedding means converting text into a numeric vector.

Example:

```text
"Machine learning is a field of AI"
-> OpenAI Embedding API
-> [0.012, -0.034, 0.88, ...]
```

In this project:

```text
document chunk text -> OpenAI Embeddings -> chunk vector
user query text -> OpenAI Embeddings -> query vector
```

Qdrant does not create embeddings by itself. It stores vectors and searches them. OpenAI creates the vectors.

### Vector Search

Vector search finds text by meaning, not only by exact keywords.

In this project, Qdrant uses cosine similarity. The idea is:

```text
similar meaning -> vectors are close
different meaning -> vectors are far apart
```

When a user asks a question, the question is embedded into a vector. Qdrant compares that query vector with stored chunk vectors and returns the nearest chunks.

### Chunking

Chunking means splitting a document into smaller text pieces before embedding.

We chunk because:

- Full documents are too large to embed or pass to the chat model efficiently.
- Smaller chunks make search more precise.
- The chat model only needs the most relevant pieces, not the entire document.

### Overlap

Overlap means each chunk shares some text with the previous or next chunk.

Example:

```text
chunk 1: tokens 1-200
chunk 2: tokens 161-360
chunk 3: tokens 321-520
```

Here the overlap is 40 tokens.

Overlap prevents losing meaning at chunk boundaries. If an important idea starts at the end of chunk 1 and continues into chunk 2, overlap helps both chunks keep enough context.

### TopK

`topK` controls how many relevant chunks are returned from vector search.

Example:

```text
topK=5
```

means Qdrant returns the 5 closest chunks.

Higher `topK` can give the model more context, but it can also add noise and increase token cost.

### Grounding

Grounding means forcing the model to answer from retrieved document context instead of general knowledge.

In this project, grounding happens through:

- semantic search retrieving relevant chunks
- system prompt saying "Answer using only the provided context"
- citations pointing back to retrieved chunks

### Retrieval

Retrieval is the step that finds relevant chunks from the vector database.

```text
question -> embedding -> Qdrant search -> relevant chunks
```

Retrieval quality is one of the most important parts of RAG. If retrieval returns poor context, the final answer will likely be poor too.

### Generation

Generation is the step where the chat model writes the final answer.

```text
question + recent history + retrieved chunks -> OpenAI Chat Model -> final answer
```

The chat model should synthesize the retrieved text into a useful answer and include citations.

### Hallucination

Hallucination means the model says something unsupported or false.

RAG reduces hallucination by giving the model retrieved document context and instructing it to answer only from that context. It does not remove hallucination completely, so cited answers and source snippets are still important.

## Supporting Flows

The main RAG flows are upload, search, and chat. These supporting flows are useful to understand when debugging or operating the project.

### Document Status Flow

```text
pending
-> parsing
-> chunked
-> embedding
-> ready
```

If something fails during parsing, embedding, database writes, or vector upsert, the document becomes:

```text
failed
```

### Document List / Status / Delete Flow

```text
list documents
-> check document status
-> delete document when needed
-> delete related vectors from Qdrant
-> remove uploaded file from disk when available
```

This flow is mostly for UI management and cleanup. It is also useful when checking why an uploaded document is not searchable yet.

### Chat Session Flow

```text
create or get chat session
-> save user message
-> generate assistant answer
-> save assistant message with citations and token usage
-> list old sessions and messages
```

Chat sessions allow the app to keep recent conversation history and reuse it in later prompts.

### Streaming Chat Flow

```text
same retrieval and generation flow as normal chat
-> return tokens gradually through SSE
-> send citations
-> send done event
-> save the final exchange
```

Streaming improves user experience because the user sees the answer as it is generated instead of waiting for the full response.

### Health Flow

```text
check API health
-> check PostgreSQL connectivity
-> check Qdrant connectivity
-> return health status
```

Health checks are useful for deployment, uptime monitoring, and quick debugging after Docker containers start.

## Concepts Still Worth Learning

The questions above cover the core concepts needed to understand this project. After that, the next useful concepts are:

- **Recall vs precision**: recall means finding enough relevant chunks; precision means avoiding irrelevant chunks.
- **Chunk size tuning**: smaller chunks improve precision, larger chunks preserve more context.
- **Embedding model choice**: different embedding models affect search quality, vector size, latency, and cost.
- **Reranking**: after vector search, a reranker can reorder results to improve relevance.
- **Hybrid search**: combines vector search with keyword search for better results on exact terms, IDs, names, and rare words.
- **Metadata filtering**: search only inside a document, user workspace, category, date range, or permission scope.
- **Prompt construction**: decides how retrieved chunks, history, and instructions are arranged before calling the chat model.
- **Context window and token budget**: controls how much retrieved text can fit into the final prompt.
- **Evaluation**: test whether retrieval finds the right chunks and whether answers are faithful to sources.
- **Observability**: log latency, topK, scores, token usage, failed jobs, and retrieval quality.

For this project, the must-know foundation is:

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
