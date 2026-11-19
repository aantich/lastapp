import { useState, type Dispatch, type SetStateAction } from "react";
import { useApiQuery } from "../query";
import type { Item } from "../api-types";
import { definitionOf } from "./shared";
import { poll } from "../polling";

export function usePublicationPoll(
  record: Item | null,
  dirty: boolean,
  setRecord: Dispatch<SetStateAction<Item | null>>,
  setDraft: Dispatch<SetStateAction<Item | null>>,
  load: () => Promise<void>,
) {
  // While a publication is pending, poll the entity every 1.5 s until the
  // pending version clears. refetchInterval never overlaps requests. Only
  // responses fetched for this publication count, never an older cached read.
  const pendingQuery = useApiQuery<Item>(
      record?.pending_version_id ? "/v1/entities/" + record.id : null,
      {
        refetchInterval: (q) =>
          q.state.data && !q.state.data.pending_version_id
            ? false
            : poll.pending,
      },
    ),
    [polled, setPolled] = useState(0);
  if (
    pendingQuery.data &&
    pendingQuery.isFetchedAfterMount &&
    pendingQuery.dataUpdatedAt !== polled
  ) {
    const v = pendingQuery.data;
    setPolled(pendingQuery.dataUpdatedAt);
    // Keep unsaved edits: take only publication state from the server.
    setRecord((current) =>
      dirty &&
      current &&
      JSON.stringify(current.draft_json) !== JSON.stringify(v.draft_json)
        ? {
            ...current,
            publications: v.publications,
            pending_version_id: v.pending_version_id,
            pending_version_number: v.pending_version_number,
            active_version_id: v.active_version_id,
            active_version_number: v.active_version_number,
            next_version_number: v.next_version_number,
            active_definition: v.active_definition,
          }
        : v,
    );
    if (!dirty) setDraft(definitionOf(v.draft_json));
    if (!v.pending_version_id) void load();
  }
  return pendingQuery;
}
