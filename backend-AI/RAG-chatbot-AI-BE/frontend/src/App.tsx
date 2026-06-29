import { useCallback, useEffect, useMemo, useState } from "react";

import { API_BASE, fetchJson } from "./api";
import { HealthBadge } from "./components/HealthBadge";
import { ChatPanel } from "./features/chat/ChatPanel";
import { DocumentsPanel } from "./features/documents/DocumentsPanel";
import { SearchPanel } from "./features/search/SearchPanel";
import { SessionsPanel } from "./features/sessions/SessionsPanel";
import type { ChatMessage, ChatSession, DocumentRecord, HealthResponse, Tab } from "./types";

const tabs: Array<{ id: Tab; label: string }> = [
  { id: "documents", label: "Documents" },
  { id: "search", label: "Search" },
  { id: "chat", label: "Chat" },
  { id: "sessions", label: "Sessions" }
];

function App() {
  const [activeTab, setActiveTab] = useState<Tab>("documents");
  const [documents, setDocuments] = useState<DocumentRecord[]>([]);
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [selectedSessionId, setSelectedSessionId] = useState("");
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [statusFilter, setStatusFilter] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");

  const refreshHealth = useCallback(async () => {
    try {
      const response = await fetch(`${API_BASE}/health`);
      const body = (await response.json()) as HealthResponse;
      setHealth(body);
    } catch (error) {
      setHealth({
        status: "down",
        service: "rag-chatbot-ai-backend",
        timestamp: new Date().toISOString(),
        dependencies: {
          api: { status: "down", error: error instanceof Error ? error.message : "unknown error" }
        }
      });
    }
  }, []);

  const refreshDocuments = useCallback(async () => {
    const query = new URLSearchParams({ page: "1", limit: "50" });
    if (statusFilter) {
      query.set("status", statusFilter);
    }
    const data = await fetchJson<{ documents: DocumentRecord[] }>(`/documents?${query.toString()}`);
    setDocuments(data.documents);
  }, [statusFilter]);

  const refreshSessions = useCallback(async () => {
    const data = await fetchJson<{ sessions: ChatSession[] }>("/chat/sessions?page=1&limit=50");
    setSessions(data.sessions);
  }, []);

  const loadMessages = useCallback(async (sessionId: string) => {
    if (!sessionId) {
      setMessages([]);
      return;
    }
    const data = await fetchJson<{ messages: ChatMessage[] }>(
      `/chat/sessions/${sessionId}/messages?limit=100`
    );
    setMessages(data.messages);
  }, []);

  useEffect(() => {
    void refreshHealth();
  }, [refreshHealth]);

  useEffect(() => {
    void refreshDocuments().catch((error) => {
      setNotice(error instanceof Error ? error.message : "Failed to load documents.");
    });
  }, [refreshDocuments]);

  useEffect(() => {
    if (activeTab === "sessions" || activeTab === "chat") {
      void refreshSessions().catch((error) => {
        setNotice(error instanceof Error ? error.message : "Failed to load sessions.");
      });
    }
  }, [activeTab, refreshSessions]);

  useEffect(() => {
    const hasProcessing = documents.some((document) =>
      ["pending", "parsing", "chunked", "embedding"].includes(document.status)
    );
    if (!hasProcessing) {
      return;
    }
    const id = window.setInterval(() => {
      void refreshDocuments().catch((error) => {
        setNotice(error instanceof Error ? error.message : "Failed to refresh documents.");
      });
    }, 2500);
    return () => window.clearInterval(id);
  }, [documents, refreshDocuments]);

  const readyDocuments = useMemo(
    () => documents.filter((document) => document.status === "ready").length,
    [documents]
  );

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <h1>RAG Console</h1>
          <p>
            {documents.length} documents · {readyDocuments} ready
          </p>
        </div>
        <HealthBadge health={health} onRefresh={refreshHealth} />
      </header>

      <nav className="tabs" aria-label="Primary">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            className={activeTab === tab.id ? "tab active" : "tab"}
            type="button"
            onClick={() => setActiveTab(tab.id)}
          >
            {tab.label}
          </button>
        ))}
      </nav>

      {notice ? <div className="notice">{notice}</div> : null}

      {activeTab === "documents" ? (
        <DocumentsPanel
          documents={documents}
          statusFilter={statusFilter}
          busy={busy}
          setBusy={setBusy}
          setNotice={setNotice}
          setStatusFilter={setStatusFilter}
          refreshDocuments={refreshDocuments}
          fetchJson={fetchJson}
        />
      ) : null}

      {activeTab === "search" ? <SearchPanel fetchJson={fetchJson} /> : null}

      {activeTab === "chat" ? (
        <ChatPanel
          fetchJson={fetchJson}
          sessions={sessions}
          selectedSessionId={selectedSessionId}
          setSelectedSessionId={setSelectedSessionId}
          refreshSessions={refreshSessions}
        />
      ) : null}

      {activeTab === "sessions" ? (
        <SessionsPanel
          sessions={sessions}
          messages={messages}
          selectedSessionId={selectedSessionId}
          setSelectedSessionId={setSelectedSessionId}
          loadMessages={loadMessages}
          refreshSessions={refreshSessions}
        />
      ) : null}
    </main>
  );
}

export default App;
