import type { ChatMessage, ChatSession } from "../../types";
import { formatDate } from "../../utils/format";

type SessionsPanelProps = {
  sessions: ChatSession[];
  messages: ChatMessage[];
  selectedSessionId: string;
  setSelectedSessionId: (id: string) => void;
  loadMessages: (id: string) => Promise<void>;
  refreshSessions: () => Promise<void>;
};

export function SessionsPanel({
  sessions,
  messages,
  selectedSessionId,
  setSelectedSessionId,
  loadMessages,
  refreshSessions
}: SessionsPanelProps) {
  async function selectSession(id: string) {
    setSelectedSessionId(id);
    await loadMessages(id);
  }

  return (
    <section className="grid two">
      <div className="panel">
        <div className="panel-head">
          <h2>Sessions</h2>
          <button type="button" onClick={refreshSessions}>
            Refresh
          </button>
        </div>
        <div className="list">
          {sessions.map((session) => (
            <button
              key={session.id}
              className={selectedSessionId === session.id ? "list-item selected" : "list-item"}
              type="button"
              onClick={() => void selectSession(session.id)}
            >
              <span>{session.title || "Untitled session"}</span>
              <small>{formatDate(session.updated_at)}</small>
            </button>
          ))}
        </div>
      </div>
      <div className="panel">
        <h2>Messages</h2>
        <div className="messages">
          {messages.map((message) => (
            <article key={message.id} className={`message ${message.role}`}>
              <strong>{message.role}</strong>
              <p>{message.content}</p>
            </article>
          ))}
          {messages.length === 0 ? <p className="empty">Select a session.</p> : null}
        </div>
      </div>
    </section>
  );
}
