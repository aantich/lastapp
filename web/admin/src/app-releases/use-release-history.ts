import { useInfiniteQuery } from "@tanstack/react-query";
import { api } from "../api";
import { apiKey } from "../query";
import type { Deployment, Page, ReleaseItem } from "../api-types";

export function useReleaseHistory(app: unknown) {
  // Release history pages accumulate behind "Load more".
  const releasesPath = `/v1/apps/${app}/releases`;
  const releasePages = useInfiniteQuery({
    queryKey: [...apiKey(releasesPath), "pages"],
    queryFn: ({ pageParam, signal }) =>
      api<Page<ReleaseItem>>(
        `${releasesPath}?page=${pageParam}`,
        "GET",
        undefined,
        undefined,
        signal,
      ),
    initialPageParam: 1,
    getNextPageParam: (last, _, page) =>
      last.pagination?.has_more ? page + 1 : undefined,
  });
  const releases = (releasePages.data?.pages || []).reduce<ReleaseItem[]>(
      (all, p) => [
        ...all.filter((r) => !p.items.some((n) => n.id === r.id)),
        ...p.items,
      ],
      [],
    ),
    hasMore = !!releasePages.hasNextPage,
    listError = releasePages.error?.message || "";
  return { releasePages, releases, hasMore, listError };
}

// A failed release was never active, so the production App's list (only
// releases that went live there) does not have it: show it from the
// deployment.
export function listedReleases(
  releases: ReleaseItem[],
  deployment: Deployment | null | undefined,
  development: boolean,
): ReleaseItem[] {
  return deployment &&
    !development &&
    !releases.some((r) => r.id === deployment.release_id)
    ? [
        {
          id: deployment.release_id,
          display_name: deployment.release_name,
          build_id: "",
          created_at: deployment.created_at,
          created_by_name: deployment.deployed_by,
          agent_client_name: null,
          snapshot_hash: "",
        },
        ...releases,
      ]
    : releases;
}
