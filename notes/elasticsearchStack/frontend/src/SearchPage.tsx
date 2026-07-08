import { FormEvent, useState } from "react";
import { api, Product, SearchParams } from "./api";

// Trang Search — đọc từ Elasticsearch qua backend (không đụng SQL).
export function SearchPage() {
  const [f, setF] = useState<SearchParams>({});
  const [items, setItems] = useState<Product[]>([]);
  const [total, setTotal] = useState<number | null>(null);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);

  const set = (k: keyof SearchParams, v: string) => setF((s) => ({ ...s, [k]: v }));

  const run = async (e?: FormEvent) => {
    e?.preventDefault();
    setLoading(true);
    try {
      const r = await api.search(f);
      setItems(r.items);
      setTotal(r.total);
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
      setItems([]);
      setTotal(null);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <form onSubmit={run}>
        <input
          placeholder="Tìm kiếm (name, description)…"
          value={f.q || ""}
          onChange={(e) => set("q", e.target.value)}
          style={{ marginBottom: 10 }}
        />
        <div className="filters">
          <div>
            <label>Category</label>
            <input value={f.category || ""} onChange={(e) => set("category", e.target.value)} placeholder="laptop…" />
          </div>
          <div>
            <label>Brand</label>
            <input value={f.brand || ""} onChange={(e) => set("brand", e.target.value)} placeholder="Apple…" />
          </div>
          <div>
            <label>Status</label>
            <select value={f.status || ""} onChange={(e) => set("status", e.target.value)}>
              <option value="">(tất cả)</option>
              <option value="active">active</option>
              <option value="discontinued">discontinued</option>
            </select>
          </div>
          <div>
            <label>In stock</label>
            <select value={f.in_stock || ""} onChange={(e) => set("in_stock", e.target.value)}>
              <option value="">(tất cả)</option>
              <option value="true">còn hàng</option>
              <option value="false">hết hàng</option>
            </select>
          </div>
          <div>
            <label>Giá từ</label>
            <input type="number" value={f.min_price || ""} onChange={(e) => set("min_price", e.target.value)} />
          </div>
          <div>
            <label>Giá đến</label>
            <input type="number" value={f.max_price || ""} onChange={(e) => set("max_price", e.target.value)} />
          </div>
        </div>
        <button disabled={loading}>{loading ? "Đang tìm…" : "Tìm"}</button>
      </form>

      {err && <div className="err">{err}</div>}
      {total !== null && <p className="hint">{total} kết quả</p>}

      <div className="grid">
        {items.map((p) => (
          <div className="card" key={p.id}>
            <div className="row1">
              <span className="name">{p.name}</span>
              <span className="price">${p.price.toLocaleString()}</span>
            </div>
            <div className="desc">{p.description}</div>
            <div className="meta">
              <span className="tag">{p.category}</span>
              <span className="tag">{p.brand}</span>
              <span className="tag">{p.status}</span>
              <span className="tag">{p.in_stock ? "còn hàng" : "hết hàng"}</span>
              <span>· id {p.id} · {p.sku}</span>
            </div>
          </div>
        ))}
        {total === 0 && <p className="hint">Không có kết quả nào.</p>}
      </div>
    </div>
  );
}
