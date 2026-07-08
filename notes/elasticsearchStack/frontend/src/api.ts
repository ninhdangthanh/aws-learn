// Lớp gọi backend Go. Search đọc ES; CRUD/list ghi/đọc Postgres.
const BASE = (import.meta.env.VITE_API_BASE as string) || "http://localhost:8090";

// Tenant context (6.6): gửi kèm header X-Tenant-ID để backend ÉP access filter.
// Backend không bao giờ lấy tenant từ body -> đổi ở đây chỉ mô phỏng "đang đăng nhập tenant nào".
let TENANT = "default";
export function setTenant(t: string) {
  TENANT = t || "default";
}
export function getTenant() {
  return TENANT;
}

export interface Product {
  id: number;
  tenant_id?: string;
  name: string;
  description?: string;
  sku: string;
  status: string;
  category: string;
  brand: string;
  price: number;
  in_stock: boolean;
  created_at?: string;
  updated_at?: string;
  // Phase 6: đoạn highlight đã escape HTML sẵn ở backend (an toàn để render).
  highlight?: { name?: string; description?: string };
}

export type ProductInput = Omit<Product, "id" | "tenant_id" | "created_at" | "updated_at" | "highlight">;

export interface Facet {
  key: string;
  count: number;
}

export interface SearchResult {
  total: number;
  total_relation: "eq" | "gte";
  size: number;
  from: number;
  items: Product[];
  facets: { brand: Facet[]; category: Facet[] };
  fallback: "none" | "fuzzy" | "suggest";
  did_you_mean: string;
  next_search_after: unknown[] | null;
  tenant: string;
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
    headers: { "Content-Type": "application/json", "X-Tenant-ID": TENANT },
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
  size?: string;
  from?: string;
  search_after?: string;
  track_total_hits?: string;
}

export const api = {
  search(p: SearchParams): Promise<SearchResult> {
    const qs = new URLSearchParams();
    Object.entries(p).forEach(([k, v]) => v && qs.set(k, v));
    return req<SearchResult>(`/search?${qs.toString()}`);
  },
  suggest(q: string): Promise<{ suggestions: string[]; tenant: string }> {
    return req(`/suggest?q=${encodeURIComponent(q)}`);
  },
  list(limit = 50): Promise<{ items: Product[]; count: number; tenant: string }> {
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
