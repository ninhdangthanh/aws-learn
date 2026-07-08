import { useState } from "react";
import { StatusBar } from "./StatusBar";
import { SearchPage } from "./SearchPage";
import { AdminPage } from "./AdminPage";

type Tab = "search" | "admin";

export function App() {
  const [tab, setTab] = useState<Tab>("search");
  // Tăng key mỗi lần CRUD -> StatusBar reconcile lại ngay.
  const [refreshKey, setRefreshKey] = useState(0);
  const bump = () => setRefreshKey((k) => k + 1);

  return (
    <div className="container">
      <h1>Elasticsearch Stack — Search + Admin</h1>
      <p className="sub">
        Search đọc <b>Elasticsearch</b>; Admin ghi <b>Postgres</b> (source of truth) rồi
        outbox worker sync sang ES. Tạo/sửa ở Admin và xem Search cập nhật sau ~1s.
      </p>

      <StatusBar refreshKey={refreshKey} />

      <div className="tabs">
        <button className={"tab" + (tab === "search" ? " active" : "")} onClick={() => setTab("search")}>
          Search (ES)
        </button>
        <button className={"tab" + (tab === "admin" ? " active" : "")} onClick={() => setTab("admin")}>
          Admin CRUD (Postgres)
        </button>
      </div>

      {tab === "search" ? <SearchPage /> : <AdminPage onChanged={bump} />}
    </div>
  );
}
