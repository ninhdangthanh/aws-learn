import { FormEvent, useCallback, useEffect, useState } from "react";
import { api, Product, ProductInput } from "./api";

const EMPTY: ProductInput = {
  name: "", description: "", sku: "", status: "active",
  category: "", brand: "", price: 0, in_stock: true,
};

// Trang Admin — CRUD ghi Postgres (source of truth). List đọc SQL nên phản ánh ngay.
// Search (tab kia) đọc ES nên trễ ~1s do outbox worker — đó chính là bài học sync.
export function AdminPage({ onChanged }: { onChanged: () => void }) {
  const [items, setItems] = useState<Product[]>([]);
  const [form, setForm] = useState<ProductInput>(EMPTY);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    try {
      const r = await api.list(50);
      setItems(r.items || []);
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const set = <K extends keyof ProductInput>(k: K, v: ProductInput[K]) =>
    setForm((s) => ({ ...s, [k]: v }));

  const reset = () => { setForm(EMPTY); setEditingId(null); };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      if (editingId == null) await api.create(form);
      else await api.update(editingId, form);
      reset();
      await load();
      onChanged();
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  const edit = (p: Product) => {
    setEditingId(p.id);
    setForm({
      name: p.name, description: p.description, sku: p.sku, status: p.status,
      category: p.category, brand: p.brand, price: p.price, in_stock: p.in_stock,
    });
  };

  const remove = async (id: number) => {
    if (!confirm(`Xóa product #${id}?`)) return;
    try {
      await api.remove(id);
      if (editingId === id) reset();
      await load();
      onChanged();
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  return (
    <div>
      <form className="card" onSubmit={submit} style={{ marginBottom: 20 }}>
        <b>{editingId == null ? "Tạo product" : `Sửa product #${editingId}`}</b>
        <div className="formgrid" style={{ marginTop: 10 }}>
          <div className="full">
            <label>Name *</label>
            <input required value={form.name} onChange={(e) => set("name", e.target.value)} />
          </div>
          <div className="full">
            <label>Description</label>
            <textarea rows={2} value={form.description} onChange={(e) => set("description", e.target.value)} />
          </div>
          <div>
            <label>SKU</label>
            <input value={form.sku} onChange={(e) => set("sku", e.target.value)} />
          </div>
          <div>
            <label>Category</label>
            <input value={form.category} onChange={(e) => set("category", e.target.value)} />
          </div>
          <div>
            <label>Brand</label>
            <input value={form.brand} onChange={(e) => set("brand", e.target.value)} />
          </div>
          <div>
            <label>Price</label>
            <input type="number" step="0.01" value={form.price}
              onChange={(e) => set("price", parseFloat(e.target.value) || 0)} />
          </div>
          <div>
            <label>Status</label>
            <select value={form.status} onChange={(e) => set("status", e.target.value)}>
              <option value="active">active</option>
              <option value="discontinued">discontinued</option>
            </select>
          </div>
          <div>
            <label>In stock</label>
            <select value={String(form.in_stock)} onChange={(e) => set("in_stock", e.target.value === "true")}>
              <option value="true">còn hàng</option>
              <option value="false">hết hàng</option>
            </select>
          </div>
        </div>
        <div className="actions" style={{ marginTop: 12 }}>
          <button type="submit">{editingId == null ? "Tạo" : "Lưu"}</button>
          {editingId != null && <button type="button" className="ghost" onClick={reset}>Hủy</button>}
        </div>
      </form>

      {err && <div className="err">{err}</div>}

      <table>
        <thead>
          <tr><th>ID</th><th>Name</th><th>Category</th><th>Brand</th><th>Price</th><th>Status</th><th></th></tr>
        </thead>
        <tbody>
          {items.map((p) => (
            <tr key={p.id}>
              <td>{p.id}</td>
              <td>{p.name}</td>
              <td>{p.category}</td>
              <td>{p.brand}</td>
              <td>${p.price.toLocaleString()}</td>
              <td>{p.status}</td>
              <td className="actions">
                <button className="ghost" onClick={() => edit(p)}>Sửa</button>
                <button className="danger" onClick={() => remove(p.id)}>Xóa</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {items.length === 0 && <p className="hint">Chưa có product. Tạo mới ở form trên.</p>}
    </div>
  );
}
