import { FormEvent, useState } from "react";

import { streamChat } from "../../api";
import { CitationList } from "../../components/CitationList";
import type { ChatResponse, ChatSession, Citation, FetchJson, TokenUsage } from "../../types";

type ChatPanelProps = {
  fetchJson: FetchJson;
  sessions: ChatSession[];
  selectedSessionId: string;
  setSelectedSessionId: (id: string) => void;
  refreshSessions: () => Promise<void>;
};

export function ChatPanel({
  fetchJson,
  sessions,
  selectedSessionId,
  setSelectedSessionId,
  refreshSessions
}: ChatPanelProps) {
  const [question, setQuestion] = useState("Công nghệ có thay thế giáo viên không?");
  const [answer, setAnswer] = useState("");
  const [citations, setCitations] = useState<Citation[]>([]);
  const [stream, setStream] = useState(true);
  const [loading, setLoading] = useState(false);
  const [usage, setUsage] = useState<TokenUsage | null>(null);
  const [error, setError] = useState("");

  async function ask(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setAnswer("");
    setCitations([]);
    setUsage(null);
    setError("");
    try {
      const payload = {
        question,
        session_id: selectedSessionId || undefined,
        top_k: 5,
        score_threshold: 0.3,
        stream
      };
      if (stream) {
        await streamChat(payload, setAnswer, setCitations, setSelectedSessionId, setUsage);
      } else {
        const response = await fetchJson<ChatResponse>("/chat", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload)
        });
        setAnswer(response.answer);
        setCitations(response.citations);
        setSelectedSessionId(response.session_id);
        setUsage(response.token_usage);
      }
      await refreshSessions();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Chat failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="grid two">
      <form className="panel stack" onSubmit={ask}>
        <h2>Chat</h2>
        <label>
          Session
          <select value={selectedSessionId} onChange={(event) => setSelectedSessionId(event.target.value)}>
            <option value="">New session</option>
            {sessions.map((session) => (
              <option key={session.id} value={session.id}>
                {session.title || session.id}
              </option>
            ))}
          </select>
        </label>
        <label>
          Question
          <textarea value={question} onChange={(event) => setQuestion(event.target.value)} />
        </label>
        <label className="check">
          <input checked={stream} type="checkbox" onChange={(event) => setStream(event.target.checked)} />
          Stream response
        </label>
        <button className="primary" disabled={loading} type="submit">
          Ask
        </button>
        {error ? <p className="error-text">{error}</p> : null}
      </form>
      <div className="panel">
        <h2>Answer</h2>
        <div className="answer">{answer || "No answer yet."}</div>
        {usage ? <p className="muted">Tokens: {usage.total_tokens}</p> : null}
        <CitationList citations={citations} />
      </div>
    </section>
  );
}
