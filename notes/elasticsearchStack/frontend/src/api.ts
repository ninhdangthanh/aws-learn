// Lớp gọi backend Go. Search đọc ES; CRUD/list ghi/đọc Postgres.
const BASE = (import.meta.env.VITE_API_BASE as string) || "http://localhost:8090";

export interface Product {
  id: number;
  name: string;
  description: string;
  sku: string;
  status: string;
  category: string;
  brand: string;
  price: number;
  in_stock: boolean;
  created_at?: string;
  updated_at?: string;
}

export type ProductInput = Omit<Product, "id" | "created_at" | "updated_at">;

export interface SearchResult {
  total: number;
  size: number;
  from: number;
  items: Product[];
}

export interface Reconcile {
  postgres: number;
  elasticsearch: number | string;
  outbox_pending: number;
  drift?: number;
  in_sync: boolean;
  es_error?: string;
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status} ${res.statusText}: ${body}`);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export interface SearchParams {
  q?: string;
  category?: string;
  status?: string;
  brand?: string;
  in_stock?: string;
  min_price?: string;
  max_price?: string;
}

export const api = {
  search(p: SearchParams): Promise<SearchResult> {
    const qs = new URLSearchParams();
    Object.entries(p).forEach(([k, v]) => v && qs.set(k, v));
    return req<SearchResult>(`/search?${qs.toString()}`);
  },
  list(limit = 50): Promise<{ items: Product[]; count: number }> {
    return req(`/products?limit=${limit}`);
  },
  create(input: ProductInput): Promise<Product> {
    return req(`/products`, { method: "POST", body: JSON.stringify(input) });
  },
  update(id: number, input: ProductInput): Promise<Product> {
    return req(`/products/${id}`, { method: "PUT", body: JSON.stringify(input) });
  },
  remove(id: number): Promise<void> {
    return req(`/products/${id}`, { method: "DELETE" });
  },
  reconcile(): Promise<Reconcile> {
    return req(`/admin/reconcile`);
  },
  backfill(): Promise<{ backfilled: number }> {
    return req(`/admin/backfill`, { method: "POST" });
  },
};
