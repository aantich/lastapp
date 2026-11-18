import type { Item, Page } from "../../api-types";
import { queryClient } from "../../query";

// Serves management list paths from story data with the console's paging
// contract (?page, ?limit, ?q → { items, pagination: { has_more } }), so
// Collection and RemoteSelect run unchanged in Storybook. Any other query
// parameter filters rows by that field's exact value. Returns a restore
// function for a story's beforeEach.
export function mockApi(
  lists: Record<string, Item[]>,
  { delay = 250, pageSize = 5 } = {},
) {
  const original = window.fetch;
  queryClient.clear();
  window.fetch = async (input, init) => {
    const url = new URL(String(input), location.href);
    await new Promise((resolve) => setTimeout(resolve, delay));
    if ((init?.method || "GET") !== "GET")
      return Response.json({ ok: true }, { status: 200 });
    const rows = lists[url.pathname];
    if (!rows)
      return Response.json(
        { error: { code: "not_found", message: "Not found in story data." } },
        { status: 404 },
      );
    const q = (url.searchParams.get("q") || "").toLowerCase();
    const page = Number(url.searchParams.get("page") || 1);
    const limit = Math.min(
      Number(url.searchParams.get("limit") || pageSize),
      pageSize,
    );
    const exact = [...url.searchParams].filter(
      ([key]) => !["q", "page", "limit"].includes(key),
    );
    const found = rows.filter(
      (row) =>
        exact.every(([key, value]) => String(row[key]) === value) &&
        Object.values(row).some((v) => String(v).toLowerCase().includes(q)),
    );
    const body: Page = {
      items: found.slice((page - 1) * limit, page * limit),
      pagination: { page, limit, has_more: page * limit < found.length },
    };
    return Response.json(body);
  };
  return () => {
    window.fetch = original;
    queryClient.clear();
  };
}
