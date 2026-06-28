import type { SearchMatch } from "../types";

export function ResultsList({ results }: { results: SearchMatch[] }) {
  return (
    <div className="panel">
      <h2>Results</h2>
      <div className="results">
        {results.map((result) => (
          <article key={result.chunk_id} className="result">
            <div className="result-meta">
              <span>{result.filename}</span>
              <span>page {result.page_number ?? "?"}</span>
              <span>{result.score.toFixed(3)}</span>
            </div>
            <p>{result.text}</p>
          </article>
        ))}
        {results.length === 0 ? <p className="empty">No results yet.</p> : null}
      </div>
    </div>
  );
}
