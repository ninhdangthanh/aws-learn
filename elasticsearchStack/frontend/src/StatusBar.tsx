import { useCallback, useEffect, useState } from "react";
import { api, Reconcile } from "./api";

// Thanh trạng thái đối soát: Postgres vs ES + outbox pending. Poll mỗi 3s.
export function StatusBar({ refreshKey }: { refreshKey: number }) {
  const [r, setR] = useState<Reconcile | null>(null);
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    try {
      setR(await api.reconcile());
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
    }
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, 3000);
    return () => clearInterval(t);
  }, [load, refreshKey]);

  const doBackfill = async () => {
    await api.backfill();
    load();
  };

  return (
    <div className="statusbar">
      <b>Sync</b>
      {err && <span className="badge bad">backend lỗi: {err}</span>}
      {r && (
        <>
          <span className={"badge " + (r.in_sync ? "ok" : "bad")}>
            {r.in_sync ? "IN SYNC" : "DRIFT"}
          </span>
          <span className="pill">Postgres <b>{r.postgres}</b></span>
          <span className="pill">ES <b>{String(r.elasticsearch)}</b></span>
          <span className="pill">outbox pending <b>{r.outbox_pending}</b></span>
          {!r.in_sync && (
            <button className="ghost" onClick={doBackfill}>Backfill từ DB</button>
          )}
        </>
      )}
    </div>
  );
}
