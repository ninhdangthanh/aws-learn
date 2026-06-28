import type { Citation } from "../types";

export function CitationList({ citations }: { citations: Citation[] }) {
  if (citations.length === 0) {
    return null;
  }
  return (
    <div className="citations">
      <h3>Citations</h3>
      {citations.map((citation) => (
        <article key={citation.chunk_id}>
          <strong>{citation.filename}</strong>
          <span>page {citation.page_number ?? "?"} · score {citation.score.toFixed(3)}</span>
          <p>{citation.text_snippet}</p>
        </article>
      ))}
    </div>
  );
}
