import React, {useEffect, useRef, useState} from "react";
import Link from "@docusaurus/Link";
import {useHistory} from "@docusaurus/router";
import styles from "./SidebarSearch.module.css";

type SearchPage = {i: number; t: string; u: string; b?: string[]};
type SearchIndex = Array<{documents: SearchPage[]}>;

function normalized(value: string): string {
  return value.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase();
}

function suggestionsFor(pages: SearchPage[], query: string): SearchPage[] {
  const terms = normalized(query).trim().split(/\s+/).filter(Boolean);
  if (!terms.length) return [];
  return pages
    .map((page) => {
      const title = normalized(page.t);
      const context = normalized((page.b ?? []).join(" "));
      if (!terms.every((term) => title.includes(term) || context.includes(term))) return null;
      const score = (title.startsWith(terms[0]) ? 0 : title.includes(terms[0]) ? 1 : 3)
        + (page.u.includes("/api/endpoints/") ? 2 : 0);
      return {page, score};
    })
    .filter((item): item is {page: SearchPage; score: number} => item !== null)
    .sort((a, b) => a.score - b.score || a.page.t.localeCompare(b.page.t))
    .slice(0, 8)
    .map((item) => item.page);
}

export default function SidebarSearch(): React.JSX.Element {
  const history = useHistory();
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const loadedRef = useRef(false);
  const [query, setQuery] = useState("");
  const [pages, setPages] = useState<SearchPage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState({top: 0, left: 0});
  const results = suggestionsFor(pages, query);
  const trimmedQuery = query.trim();

  useEffect(() => {
    const onShortcut = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        inputRef.current?.focus();
      }
    };
    const onOutside = (event: PointerEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("keydown", onShortcut);
    document.addEventListener("pointerdown", onOutside);
    return () => {
      document.removeEventListener("keydown", onShortcut);
      document.removeEventListener("pointerdown", onOutside);
    };
  }, []);

  function placeResults() {
    const rect = inputRef.current?.getBoundingClientRect();
    if (rect) setPosition({top: rect.bottom + 8, left: rect.left});
  }

  async function loadPages() {
    if (loadedRef.current || loading) return;
    setLoading(true);
    setError(false);
    try {
      const response = await fetch("/search-index.json");
      if (!response.ok) throw new Error(`Search index: ${response.status}`);
      const index = await response.json() as SearchIndex;
      setPages(index[0]?.documents ?? []);
      loadedRef.current = true;
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }

  function openResults() {
    if (!trimmedQuery) return;
    setOpen(false);
    history.push(`/search/?q=${encodeURIComponent(trimmedQuery)}`);
  }

  return (
    <div className={styles.search} ref={rootRef}>
      <div className={styles.field}>
        <svg aria-hidden="true" viewBox="0 0 20 20"><circle cx="8.5" cy="8.5" r="5.5"/><path d="m13 13 4.5 4.5"/></svg>
        <input
          ref={inputRef}
          type="search"
          aria-label="Search documentation"
          aria-controls="akoflow-search-suggestions"
          aria-expanded={open && !!trimmedQuery}
          aria-autocomplete="list"
          role="combobox"
          placeholder="Search docs…"
          value={query}
          onFocus={() => { placeResults(); setOpen(true); void loadPages(); }}
          onChange={(event) => { setQuery(event.target.value); placeResults(); setOpen(true); }}
          onKeyDown={(event) => {
            if (event.key === "Enter") { event.preventDefault(); openResults(); }
            if (event.key === "Escape") { setOpen(false); inputRef.current?.blur(); }
          }}
        />
        {!query && <kbd aria-hidden="true">⌘K</kbd>}
      </div>
      {open && trimmedQuery && (
        <div id="akoflow-search-suggestions" className={styles.results} role="listbox" style={position}>
          {loading && <p className={styles.status}>Loading search index…</p>}
          {error && <p className={styles.status}>Suggestions unavailable. You can still open full search.</p>}
          {!loading && !error && !results.length && <p className={styles.status}>No page titles found. Try full search for page content.</p>}
          {!loading && !error && results.map((page) => (
            <Link key={page.i} className={styles.result} to={page.u} role="option" aria-selected="false" onClick={() => setOpen(false)}>
              <strong>{page.t}</strong>
              <small>{(page.b ?? []).join(" › ")}</small>
            </Link>
          ))}
          <button type="button" className={styles.allResults} onClick={openResults}>
            See all results for “{trimmedQuery}” <span aria-hidden="true">→</span>
          </button>
        </div>
      )}
    </div>
  );
}
