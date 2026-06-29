# Frontend Notes

This folder contains the Phase 12 React UI for the RAG chatbot backend.

## Quick Start

```bash
npm install
npm run dev
```

The dev server runs at `http://localhost:5173`.

By default the UI calls `/api/v1` on the same origin. Vite proxies `/api` to `http://localhost:8099` in development. For another API host, set:

```bash
VITE_API_BASE_URL=http://localhost:8099/api/v1 npm run dev
```

## File Map

```text
src/
  App.tsx                         App-level state, tab switching, shared refresh effects
  api.ts                          API base URL, JSON fetch helper, SSE chat stream parser
  types.ts                        Backend response/request-adjacent TypeScript types
  main.tsx                        React bootstrap
  styles.css                      Global responsive UI styles
  components/
    CitationList.tsx              Shared citation rendering
    HealthBadge.tsx               Top-right backend health badge
    ResultsList.tsx               Shared semantic search result rendering
  features/
    documents/DocumentsPanel.tsx  Upload, document list, status filter, delete
    search/SearchPanel.tsx        Semantic search form and result state
    chat/ChatPanel.tsx            Chat form, normal JSON answer, streaming answer
    sessions/SessionsPanel.tsx    Session list and historical messages
  utils/
    format.ts                     Shared date formatting
```

## Backend Contract

The UI expects these backend routes:

```text
GET    /api/v1/health
POST   /api/v1/documents
GET    /api/v1/documents?page=1&limit=50&status=ready
DELETE /api/v1/documents/:id
POST   /api/v1/search
POST   /api/v1/chat
GET    /api/v1/chat/sessions?page=1&limit=50
GET    /api/v1/chat/sessions/:id/messages?limit=100
```

`POST /chat` uses JSON by default. When `stream: true`, it returns Server-Sent Events where each event is written as:

```text
data: {"type":"token","content":"...","session_id":"..."}
data: {"type":"citations","citations":[...],"session_id":"..."}
data: {"type":"done","token_usage":{"total_tokens":123},"session_id":"..."}
data: {"type":"error","content":"..."}
```

The stream parsing lives in `src/api.ts`.

## State Ownership

`App.tsx` owns cross-tab state:

- `documents`: used by the topbar and Documents tab.
- `sessions`: shared by Chat and Sessions tabs.
- `messages`: loaded only for the selected session in Sessions.
- `selectedSessionId`: shared by Chat and Sessions.
- `health`: used by the health badge.
- `notice`: shared top-level operational notice.

Each feature panel owns its local form state, loading state, and inline error state.

## Common Changes

- Add or change an API response shape: update `src/types.ts` first.
- Change API error behavior or SSE parsing: update `src/api.ts`.
- Change upload/list/delete UI: update `src/features/documents/DocumentsPanel.tsx`.
- Change search form/result behavior: update `src/features/search/SearchPanel.tsx`.
- Change chat behavior: update `src/features/chat/ChatPanel.tsx`.
- Change session history UI: update `src/features/sessions/SessionsPanel.tsx`.
- Change visual layout: update `src/styles.css`.

## Verification

Run this before handing off frontend changes:

```bash
npm run build
```

Recommended full-project check from the repository root:

```bash
GOCACHE=/private/tmp/rag-chatbot-go-cache go test ./...
```

## Known Local Gotcha

If the UI shows `Request failed with 404` for Documents while `/api/v1/health` works, the backend process is probably an older running binary. Restart the API from the current codebase so the Phase 11/12 routes are registered.
