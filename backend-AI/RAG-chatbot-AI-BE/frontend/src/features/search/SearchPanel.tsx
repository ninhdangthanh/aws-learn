import { FormEvent, useState } from "react";

import { ResultsList } from "../../components/ResultsList";
import type { FetchJson, SearchMatch } from "../../types";

export function SearchPanel({ fetchJson }: { fetchJson: FetchJson }) {
  const [query, setQuery] = useState("chiến lược dài hạn");
  const [topK, setTopK] = useState(5);
  const [threshold, setThreshold] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [results, setResults] = useState<SearchMatch[]>([]);

  async function search(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      const response = await fetchJson<{ results: SearchMatch[] }>("/search", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          query,
          top_k: topK,
          score_threshold: threshold ? Number(threshold) : undefined
        })
      });
      setResults(response.results);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Search failed.");
      setResults([]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="grid two">
      <form className="panel stack" onSubmit={search}>
        <h2>Semantic Search</h2>
        <label>
          Query
          <textarea value={query} onChange={(event) => setQuery(event.target.value)} />
        </label>
        <div className="row">
          <label>
            Top K
            <input
              min={1}
              max={20}
              type="number"
              value={topK}
              onChange={(event) => setTopK(Number(event.target.value))}
            />
          </label>
          <label>
            Threshold
            <input
              placeholder="optional"
              value={threshold}
              onChange={(event) => setThreshold(event.target.value)}
            />
          </label>
        </div>
        <button className="primary" disabled={loading} type="submit">
          Search
        </button>
        {error ? <p className="error-text">{error}</p> : null}
      </form>
      <ResultsList results={results} />
    </section>
  );
}
