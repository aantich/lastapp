import { useState } from "react";
import { ArrowUpRight, Search, X } from "lucide-react";
import { useDialogFocus } from "./appearance";

export function NavigationSearch({
  pages,
  navigate,
  close,
}: {
  pages: Record<string, any>[];
  navigate: (page: string) => void;
  close: () => void;
}) {
  const [query, setQuery] = useState("");
  const ref = useDialogFocus(close);
  const results = pages.filter((page) =>
    page.label.toLowerCase().includes(query.toLowerCase()),
  );
  return (
    <div
      className="overlay search-overlay"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) close();
      }}
    >
      <section
        className="navigation-search panel"
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby="navigation-title"
        tabIndex={-1}
      >
        <header>
          <Search size={19} />
          <input
            data-autofocus
            aria-label="Find a screen"
            placeholder="Where would you like to go?"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && results[0]) {
                event.preventDefault();
                navigate(results[0].id);
              }
            }}
          />
          <button
            className="icon"
            aria-label="Close navigation search"
            onClick={close}
          >
            <X size={18} />
          </button>
        </header>
        <h2 id="navigation-title">Quick navigation</h2>
        <div className="navigation-results">
          {results.map(({ id, label, icon: Icon }) => (
            <button key={id} onClick={() => navigate(id)}>
              <Icon size={18} />
              <span>{label}</span>
              <ArrowUpRight size={15} />
            </button>
          ))}
          {!results.length && <p>No screens match “{query}”.</p>}
        </div>
        <footer>
          Enter to open the first result<span>Esc to close</span>
        </footer>
      </section>
    </div>
  );
}
