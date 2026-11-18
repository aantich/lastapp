import { createContext, useContext } from "react";
import {
  QueryClient,
  useQueries,
  useQuery,
  type UseQueryOptions,
} from "@tanstack/react-query";
import { api } from "./api";
import type { Item, Page } from "./api-types";

// One cache for every console read. Screens show request errors immediately
// (no silent retries) and refresh on explicit invalidation or a polling
// interval, not on window focus, so a background refetch never surprises an
// operator mid-edit.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false, refetchOnWindowFocus: false },
  },
});

// Every console GET is cached under its API path.
export const apiKey = (path: string) => ["api", path] as const;

// Marks all console reads stale and refetches the mounted ones. This replaces
// per-screen reload counters: a mutation invalidates, the visible screen
// refetches whatever it shows.
export function refreshQueries() {
  return queryClient.invalidateQueries({ queryKey: ["api"] });
}

// A subtree's live-refresh interval. Workspaces set it once; every query
// below polls at that cadence. refetchInterval waits for the previous request
// to finish, so polls never overlap, and pauses while the tab is hidden.
export const LiveRefresh = createContext<number | false>(false);

type ApiQueryOptions<T> = Omit<
  UseQueryOptions<T, Error, T, ReturnType<typeof apiKey>>,
  "queryKey" | "queryFn"
>;

// Reads `path` with the typed api() helper. A null path disables the query.
// The AbortSignal cancels the request when the query is cancelled or its
// component leaves the screen.
export function useApiQuery<T>(
  path: string | null | undefined,
  options: ApiQueryOptions<T> = {},
) {
  const live = useContext(LiveRefresh);
  return useQuery({
    refetchInterval: live,
    ...options,
    queryKey: apiKey(path || ""),
    queryFn: ({ signal }) => api<T>(path!, "GET", undefined, undefined, signal),
    enabled: !!path && options.enabled !== false,
  });
}

// Reads several paths at once (one cached query each, same keys and live
// interval as useApiQuery), e.g. one row per App.
export function useApiQueries<T>(paths: string[]) {
  const live = useContext(LiveRefresh);
  return useQueries({
    queries: paths.map((path) => ({
      refetchInterval: live,
      queryKey: apiKey(path),
      queryFn: ({ signal }: { signal: AbortSignal }) =>
        api<T>(path, "GET", undefined, undefined, signal),
    })),
  });
}

// Single reads already in the cache whose path `match` accepts, with that
// path (e.g. the App deployments Home has shown). Nothing is fetched.
export function cachedData<T>(match: (path: string) => boolean) {
  const found: [string, T][] = [];
  for (const [key, data] of queryClient.getQueriesData<T>({
    queryKey: ["api"],
  })) {
    const path = String(key[1] || "");
    if (data !== undefined && match(path)) found.push([path, data]);
  }
  return found;
}

// Rows of list reads already in the cache (Collection pages, RemoteSelect and
// ContextSwitcher searches) whose path, without its query string, `match`
// accepts, de-duplicated by id. Nothing is fetched: the command palette offers
// jumps to what this session has already been shown.
export function cachedRows(match: (path: string) => boolean): Item[] {
  const rows = new Map<string, Item>();
  for (const [key, data] of queryClient.getQueriesData({ queryKey: ["api"] })) {
    const path = String(key[1] || "").split("?")[0];
    if (!match(path) || !data || typeof data !== "object") continue;
    const pages = "pages" in data ? (data as { pages: Page[] }).pages : [data];
    for (const page of pages as Page[])
      for (const row of page?.items || [])
        if (typeof row?.id === "string") rows.set(row.id, row);
  }
  return [...rows.values()];
}
