import type { HealthResponse } from "../types";

export function HealthBadge({ health, onRefresh }: { health: HealthResponse | null; onRefresh: () => void }) {
  const state = health?.status ?? "checking";
  return (
    <button className={`health ${state}`} type="button" onClick={onRefresh} title="Refresh health">
      <span className="dot" />
      {state}
    </button>
  );
}
