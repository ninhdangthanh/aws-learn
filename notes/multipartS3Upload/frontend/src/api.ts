// Thin client for the Go backend's four multipart endpoints.
const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080";

export interface InitResponse {
  key: string;
  uploadId: string;
  partSize: number;
}

export interface PresignedPart {
  partNumber: number;
  url: string;
}

export interface CompletedPart {
  partNumber: number;
  etag: string;
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`${path} failed (${res.status}): ${text}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  init(filename: string, size: number, contentType: string) {
    return postJSON<InitResponse>("/uploads/init", { filename, size, contentType });
  },

  presignParts(key: string, uploadId: string, partNumbers: number[]) {
    return postJSON<{ parts: PresignedPart[] }>("/uploads/presign-parts", {
      key,
      uploadId,
      partNumbers,
    });
  },

  complete(key: string, uploadId: string, parts: CompletedPart[]) {
    return postJSON<{ key: string; location: string }>("/uploads/complete", {
      key,
      uploadId,
      parts,
    });
  },

  abort(key: string, uploadId: string) {
    return postJSON<{ status: string }>("/uploads/abort", { key, uploadId });
  },
};
