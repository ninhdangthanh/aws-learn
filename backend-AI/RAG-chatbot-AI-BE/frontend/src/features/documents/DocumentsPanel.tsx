import { FormEvent, useState } from "react";

import type { DocumentRecord, FetchJson } from "../../types";
import { formatDate } from "../../utils/format";

type DocumentsPanelProps = {
  documents: DocumentRecord[];
  statusFilter: string;
  busy: boolean;
  setBusy: (busy: boolean) => void;
  setNotice: (notice: string) => void;
  setStatusFilter: (status: string) => void;
  refreshDocuments: () => Promise<void>;
  fetchJson: FetchJson;
};

export function DocumentsPanel({
  documents,
  statusFilter,
  busy,
  setBusy,
  setNotice,
  setStatusFilter,
  refreshDocuments,
  fetchJson
}: DocumentsPanelProps) {
  const [file, setFile] = useState<File | null>(null);

  async function upload(event: FormEvent) {
    event.preventDefault();
    if (!file) {
      setNotice("Choose a PDF first.");
      return;
    }
    setBusy(true);
    setNotice("");
    try {
      const body = new FormData();
      body.append("file", file);
      await fetchJson("/documents", { method: "POST", body });
      setFile(null);
      setNotice("Upload accepted. Worker processing will update status automatically.");
      await refreshDocuments();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Upload failed.");
    } finally {
      setBusy(false);
    }
  }

  async function deleteDocument(id: string) {
    if (!window.confirm("Delete this document and its vectors?")) {
      return;
    }
    setBusy(true);
    setNotice("");
    try {
      await fetchJson(`/documents/${id}`, { method: "DELETE" });
      setNotice("Document deleted.");
      await refreshDocuments();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Delete failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="grid two">
      <div className="panel">
        <h2>Upload</h2>
        <form className="stack" onSubmit={upload}>
          <label>
            PDF file
            <input
              accept="application/pdf,.pdf"
              type="file"
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
            />
          </label>
          <button className="primary" disabled={busy} type="submit">
            Upload
          </button>
        </form>
      </div>

      <div className="panel wide">
        <div className="panel-head">
          <h2>Documents</h2>
          <div className="inline">
            <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
              <option value="">All</option>
              <option value="pending">Pending</option>
              <option value="parsing">Parsing</option>
              <option value="chunked">Chunked</option>
              <option value="embedding">Embedding</option>
              <option value="ready">Ready</option>
              <option value="failed">Failed</option>
            </select>
            <button type="button" onClick={refreshDocuments}>
              Refresh
            </button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>File</th>
                <th>Status</th>
                <th>Chunks</th>
                <th>Updated</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {documents.map((document) => (
                <tr key={document.id}>
                  <td>
                    <div className="file-name">{document.filename}</div>
                    <div className="muted">{document.id}</div>
                  </td>
                  <td>
                    <span className={`status ${document.status}`}>{document.status}</span>
                  </td>
                  <td>{document.chunk_count}</td>
                  <td>{formatDate(document.updated_at)}</td>
                  <td>
                    <button type="button" onClick={() => void deleteDocument(document.id)}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
              {documents.length === 0 ? (
                <tr>
                  <td colSpan={5} className="empty">
                    No documents found.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}
