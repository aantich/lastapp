import { useEffect, useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { Item, Page } from "../api-types";
import { apiKey } from "../query";
import type { ListOption } from "./listbox";

// Appends query parameters to a list path that may already carry some.
export const withParams = (path: string, params: string) =>
  path + (path.includes("?") ? "&" : "?") + params;

// The searchable, paged options of a management list API, shared by every
// popup that lists one (RemoteSelect, ContextSwitcher). Typing waits briefly
// before searching so each keystroke is not a request; each search is one
// cached result and loadMore appends its next page. Results load only while
// `open`; the AbortSignal cancels a superseded or abandoned request. Before
// the first search the caller's `initial` options are shown.
export function useRemoteOptions<T extends ListOption>({
  path,
  open,
  convert,
  initial,
}: {
  path: string;
  open: boolean;
  convert: (items: Item[]) => T[];
  initial: T[];
}) {
  const [query, setQuery] = useState(""),
    [search, setSearch] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => setSearch(query), 150);
    return () => clearTimeout(timer);
  }, [query]);
  const results = useInfiniteQuery({
    queryKey: [...apiKey(path), "search", search],
    queryFn: ({ pageParam, signal }) =>
      api<Page>(
        withParams(
          path,
          `page=${pageParam}&limit=50&q=${encodeURIComponent(search)}`,
        ),
        "GET",
        undefined,
        undefined,
        signal,
      ),
    initialPageParam: 1,
    getNextPageParam: (last, _pages, page) =>
      last.pagination?.has_more ? page + 1 : undefined,
    enabled: open,
  });
  const typing = query !== search;
  const options: T[] = typing
    ? []
    : results.data
      ? results.data.pages
          .flatMap((p) => convert(p.items))
          .filter(
            (o, i, all) => all.findIndex((x) => x.value === o.value) === i,
          )
      : search
        ? []
        : initial;
  return {
    query,
    setQuery,
    // Clears the search at once (no debounce), e.g. when the popup opens.
    reset() {
      setQuery("");
      setSearch("");
    },
    options,
    // Searching (a new query); a next page keeps the list and its button.
    busy: typing || results.isPending,
    loadingMore: results.isFetchingNextPage,
    more: !typing && results.hasNextPage,
    error: results.error?.message || "",
    // Resolves once the next page is in `options`.
    loadMore: () => results.fetchNextPage().then(() => undefined),
  };
}

export type RemoteOptions = ReturnType<typeof useRemoteOptions>;
