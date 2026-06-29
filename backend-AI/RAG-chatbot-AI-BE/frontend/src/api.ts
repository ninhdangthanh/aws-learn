import type { Citation, TokenUsage } from "./types";

export const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "/api/v1";

export async function fetchJson<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, init);
  if (!response.ok) {
    let message = `Request failed with ${response.status}`;
    try {
      const body = await response.json();
      message = body?.error?.message ?? message;
    } catch {
      // Keep the status-based fallback.
    }
    throw new Error(message);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

export async function streamChat(
  payload: unknown,
  setAnswer: (update: (previous: string) => string) => void,
  setCitations: (citations: Citation[]) => void,
  setSelectedSessionId: (id: string) => void,
  setUsage: (usage: TokenUsage) => void
) {
  const response = await fetch(`${API_BASE}/chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  if (!response.ok || !response.body) {
    let message = `Stream failed with ${response.status}`;
    try {
      const body = await response.json();
      message = body?.error?.message ?? message;
    } catch {
      // Keep the status-based fallback.
    }
    throw new Error(message);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const events = buffer.split("\n\n");
    buffer = events.pop() ?? "";
    for (const event of events) {
      const line = event.split("\n").find((item) => item.startsWith("data:"));
      if (!line) {
        continue;
      }
      const parsed = JSON.parse(line.slice(5).trim()) as {
        type: string;
        content?: string;
        citations?: Citation[];
        session_id?: string;
        token_usage?: TokenUsage;
      };
      if (parsed.type === "error") {
        throw new Error(parsed.content ?? "Stream failed.");
      }
      if (parsed.session_id) {
        setSelectedSessionId(parsed.session_id);
      }
      if (parsed.type === "token" && parsed.content) {
        setAnswer((previous) => previous + parsed.content);
      }
      if (parsed.type === "citations" && parsed.citations) {
        setCitations(parsed.citations);
      }
      if (parsed.type === "done" && parsed.token_usage) {
        setUsage(parsed.token_usage);
      }
    }
  }
}
