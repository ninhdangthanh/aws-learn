import { FormEvent, useEffect, useRef, useState } from "react";
import { api, Facet, Product, SearchParams, setTenant, getTenant } from "./api";

const PAGE = 10;

// Trang Search — đọc từ Elasticsearch qua backend (Phase 6: facet, highlight, autocomplete,
// search_after, zero-result fallback, tenant filter). Không đụng SQL.
export function SearchPage() {
  const [f, setF] = useState<SearchParams>({});
  const [items, setItems] = useState<Product[]>([]);
  const [total, setTotal] = useState<number | null>(null);
  const [relation, setRelation] = useState<"eq" | "gte">("eq");
  const [facets, setFacets] = useState<{ brand: Facet[]; category: Facet[] }>({ brand: [], category: [] });
  const [fallback, setFallback] = useState<"none" | "fuzzy" | "suggest">("none");
  const [didYouMean, setDidYouMean] = useState("");
  const [nextAfter, setNextAfter] = useState<unknown[] | null>(null);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);

  // autocomplete
  const [sugs, setSugs] = useState<string[]>([]);
  const [showSug, setShowSug] = useState(false);

  // tenant (6.6) — mô phỏng đang đăng nhập tenant nào
  const [tenant, setTenantState] = useState(getTenant());

  const set = (k: keyof SearchParams, v: string) => setF((s) => ({ ...s, [k]: v }));

  // run — tìm từ trang đầu. Khi append=true thì nối thêm trang (search_after).
  const run = async (e?: FormEvent, override?: Partial<SearchParams>, append = false) => {
    e?.preventDefault();
    setLoading(true);
    setShowSug(false);
    const params: SearchParams = { ...f, ...override, size: String(PAGE) };
    if (append && nextAfter) params.search_after = JSON.stringify(nextAfter);
    try {
      const r = await api.search(params);
      setItems((prev) => (append ? [...prev, ...r.items] : r.items));
      setTotal(r.total);
      setRelation(r.total_relation);
      setFacets(r.facets);
      setFallback(r.fallback);
      setDidYouMean(r.did_you_mean);
      setNextAfter(r.items.length < PAGE ? null : r.next_search_after);
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
      if (!append) {
        setItems([]);
        setTotal(null);
      }
    } finally {
      setLoading(false);
    }
  };

  // autocomplete: gõ tới đâu gọi /suggest tới đó (debounce 200ms).
  const sugTimer = useRef<number | undefined>(undefined);
  useEffect(() => {
    const q = f.q || "";
    window.clearTimeout(sugTimer.current);
    if (q.trim().length < 2) {
      setSugs([]);
      return;
    }
    sugTimer.current = window.setTimeout(async () => {
      try {
        const r = await api.suggest(q);
        setSugs(r.suggestions);
        setShowSug(r.suggestions.length > 0);
      } catch {
        setSugs([]);
      }
    }, 200);
    return () => window.clearTimeout(sugTimer.current);
  }, [f.q, tenant]);

  const applyTenant = (t: string) => {
    setTenant(t);
    setTenantState(t);
  };

  // click facet -> set filter rồi tìm lại từ trang đầu.
  const pickFacet = (k: "brand" | "category", val: string) => {
    const nextVal = f[k] === val ? "" : val; // click lại để bỏ chọn
    setF((s) => ({ ...s, [k]: nextVal }));
    run(undefined, { [k]: nextVal } as Partial<SearchParams>);
  };

  const pickSuggestion = (s: string) => {
    setF((prev) => ({ ...prev, q: s }));
    run(undefined, { q: s });
  };

  const activeBrand = f.brand || "";
  const activeCat = f.category || "";

  return (
    <div className="search-layout">
      {/* Facets (6.3) */}
      <aside className="facets">
        <FacetBlock title="Brand" items={facets.brand} active={activeBrand} onPick={(v) => pickFacet("brand", v)} />
        <FacetBlock title="Category" items={facets.category} active={activeCat} onPick={(v) => pickFacet("category", v)} />
      </aside>

      <div>
        <div className="tenant-row">
          <label>Tenant (X-Tenant-ID)</label>
          <select value={tenant} onChange={(e) => applyTenant(e.target.value)}>
            <option value="default">default</option>
            <option value="acme">acme</option>
            <option value="globex">globex</option>
          </select>
          <span className="hint">Backend ép filter theo tenant — đổi để thấy cách ly dữ liệu (6.6).</span>
        </div>

        <form onSubmit={(e) => run(e)} autoComplete="off">
          <div className="suggest-wrap">
            <input
              placeholder="Tìm kiếm (name, description)… thử 'notebook' để thấy synonym"
              value={f.q || ""}
              onChange={(e) => set("q", e.target.value)}
              onFocus={() => sugs.length && setShowSug(true)}
              onBlur={() => setTimeout(() => setShowSug(false), 150)}
            />
            {showSug && (
              <ul className="suggest-list">
                {sugs.map((s) => (
                  <li key={s} onMouseDown={() => pickSuggestion(s)}>
                    {s}
                  </li>
                ))}
              </ul>
            )}
          </div>

          <div className="filters">
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

        {/* Active filter chips */}
        {(activeBrand || activeCat) && (
          <div className="chips">
            {activeBrand && <button className="chip" onClick={() => pickFacet("brand", activeBrand)}>brand: {activeBrand} ✕</button>}
            {activeCat && <button className="chip" onClick={() => pickFacet("category", activeCat)}>category: {activeCat} ✕</button>}
          </div>
        )}

        {total !== null && (
          <p className="hint">
            {relation === "gte" ? `${total.toLocaleString()}+` : total.toLocaleString()} kết quả
            {fallback === "fuzzy" && " · không khớp chính xác, đã nới lỏng (fuzzy)"}
            {fallback === "suggest" && " · không có kết quả — gợi ý phổ biến bên dưới"}
          </p>
        )}

        {/* did you mean (6.5) */}
        {didYouMean && (
          <p className="hint">
            Có phải bạn tìm{" "}
            <button className="linklike" onClick={() => pickSuggestion(didYouMean)}>
              {didYouMean}
            </button>
            ?
          </p>
        )}

        <div className="grid">
          {items.map((p) => (
            <div className="card" key={p.id}>
              <div className="row1">
                <span className="name" dangerouslySetInnerHTML={{ __html: p.highlight?.name || escapeText(p.name) }} />
                <span className="price">${p.price.toLocaleString()}</span>
              </div>
              {(p.highlight?.description || p.description) && (
                <div className="desc" dangerouslySetInnerHTML={{ __html: p.highlight?.description || escapeText(p.description || "") }} />
              )}
              <div className="meta">
                <span className="tag">{p.category}</span>
                <span className="tag">{p.brand}</span>
                <span className="tag">{p.status}</span>
                <span className="tag">{p.in_stock ? "còn hàng" : "hết hàng"}</span>
                <span>· id {p.id} · {p.sku}</span>
              </div>
            </div>
          ))}
          {total === 0 && fallback !== "suggest" && <p className="hint">Không có kết quả nào.</p>}
        </div>

        {/* Pagination search_after (6.2) */}
        {nextAfter && (
          <button className="ghost" style={{ marginTop: 12 }} disabled={loading} onClick={() => run(undefined, undefined, true)}>
            {loading ? "Đang tải…" : "Tải thêm"}
          </button>
        )}
      </div>
    </div>
  );
}

function FacetBlock({ title, items, active, onPick }: { title: string; items: Facet[]; active: string; onPick: (v: string) => void }) {
  if (!items.length) return null;
  return (
    <div className="facet-block">
      <h4>{title}</h4>
      <ul>
        {items.map((b) => (
          <li key={b.key} className={active === b.key ? "active" : ""} onClick={() => onPick(b.key)}>
            <span>{b.key}</span>
            <span className="count">{b.count}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

// escapeText — item KHÔNG có highlight thì tự escape để render an toàn qua innerHTML.
function escapeText(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c] as string));
}
