import { useRef, useState } from "react";
import { multipartUpload } from "./multipartUpload";

type Phase = "idle" | "uploading" | "done" | "error" | "aborted";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / 1024 ** i).toFixed(1)} ${units[i]}`;
}

export default function App() {
  const [file, setFile] = useState<File | null>(null);
  const [phase, setPhase] = useState<Phase>("idle");
  const [loaded, setLoaded] = useState(0);
  const [total, setTotal] = useState(0);
  const [message, setMessage] = useState<string>("");
  const abortRef = useRef<AbortController | null>(null);

  const percent = total > 0 ? Math.min(100, (loaded / total) * 100) : 0;

  async function handleUpload() {
    if (!file) return;
    const controller = new AbortController();
    abortRef.current = controller;
    setPhase("uploading");
    setLoaded(0);
    setTotal(file.size);
    setMessage("");

    try {
      const result = await multipartUpload(file, {
        concurrency: 4,
        maxRetries: 3,
        signal: controller.signal,
        onProgress: (l, t) => {
          setLoaded(l);
          setTotal(t);
        },
      });
      setPhase("done");
      setMessage(`Uploaded to: ${result.key}`);
    } catch (err) {
      if (controller.signal.aborted) {
        setPhase("aborted");
        setMessage("Upload aborted.");
      } else {
        setPhase("error");
        setMessage(err instanceof Error ? err.message : String(err));
      }
    } finally {
      abortRef.current = null;
    }
  }

  function handleAbort() {
    abortRef.current?.abort();
  }

  const uploading = phase === "uploading";

  return (
    <div className="container">
      <h1>S3 Multipart Upload</h1>
      <p className="subtitle">
        Presigned URLs + parallel parts + retry. Bytes go straight from the browser to S3.
      </p>

      <label className="dropzone">
        <input
          type="file"
          disabled={uploading}
          onChange={(e) => {
            setFile(e.target.files?.[0] ?? null);
            setPhase("idle");
            setLoaded(0);
            setTotal(0);
            setMessage("");
          }}
        />
        {file ? (
          <span>
            <strong>{file.name}</strong> — {formatBytes(file.size)}
          </span>
        ) : (
          <span>Click to choose a file</span>
        )}
      </label>

      <div className="actions">
        <button onClick={handleUpload} disabled={!file || uploading}>
          {uploading ? "Uploading…" : "Upload"}
        </button>
        <button onClick={handleAbort} disabled={!uploading} className="secondary">
          Abort
        </button>
      </div>

      {(uploading || phase === "done") && (
        <div className="progress-wrap">
          <div className="progress-bar">
            <div className="progress-fill" style={{ width: `${percent}%` }} />
          </div>
          <div className="progress-label">
            {percent.toFixed(1)}% — {formatBytes(loaded)} / {formatBytes(total)}
          </div>
        </div>
      )}

      {message && (
        <p className={`message ${phase === "error" ? "error" : phase === "done" ? "ok" : ""}`}>
          {message}
        </p>
      )}
    </div>
  );
}
