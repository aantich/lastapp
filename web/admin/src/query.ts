import { createContext, useContext } from "react";
import {
  QueryClient,
  useQuery,
  type UseQueryOptions,
} from "@tanstack/react-query";
import { api } from "./api";

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
