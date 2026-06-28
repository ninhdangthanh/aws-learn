export type DocumentStatus = "pending" | "parsing" | "chunked" | "embedding" | "ready" | "failed";

export type DocumentRecord = {
  id: string;
  filename: string;
  file_size: number;
  file_type: string;
  status: DocumentStatus;
  chunk_count: number;
  error_msg: string | null;
  created_at: string;
  updated_at: string;
};

export type SearchMatch = {
  chunk_id: string;
  document_id: string;
  filename: string;
  page_number: number | null;
  chunk_index: number;
  text: string;
  score: number;
};

export type Citation = {
  chunk_id: string;
  document_id: string;
  filename: string;
  page_number: number | null;
  chunk_index: number;
  text_snippet: string;
  score: number;
};

export type TokenUsage = {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
};

export type ChatResponse = {
  answer: string;
  citations: Citation[];
  session_id: string;
  token_usage: TokenUsage;
  latency_ms: number;
};

export type ChatSession = {
  id: string;
  title: string | null;
  created_at: string;
  updated_at: string;
};

export type ChatMessage = {
  id: string;
  session_id: string;
  role: "user" | "assistant";
  content: string;
  citations: Citation[] | null;
  token_usage: TokenUsage | null;
  latency_ms: number | null;
  created_at: string;
};

export type HealthResponse = {
  status: string;
  service: string;
  timestamp: string;
  dependencies: Record<string, { status: string; error?: string }>;
};

export type FetchJson = <T>(path: string, init?: RequestInit) => Promise<T>;

export type Tab = "documents" | "search" | "chat" | "sessions";
