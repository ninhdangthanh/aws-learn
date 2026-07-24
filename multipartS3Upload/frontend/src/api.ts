import axios, { AxiosError } from "axios";

// Thin client for the Go backend's four multipart endpoints.
const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080";

const http = axios.create({
  baseURL: API_BASE,
  headers: { "Content-Type": "application/json" },
});

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
  try {
    const res = await http.post<T>(path, body);
    return res.data;
  } catch (err) {
    if (err instanceof AxiosError && err.response) {
      const detail =
        typeof err.response.data === "string"
          ? err.response.data
          : JSON.stringify(err.response.data);
      throw new Error(`${path} failed (${err.response.status}): ${detail}`);
    }
    throw err;
  }
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
