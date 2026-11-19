import { useState, type Dispatch, type SetStateAction } from "react";
import { useApiQuery } from "../query";
import type { Item } from "../api-types";
import { definitionOf } from "./shared";
import { entityTabs } from "./entity-tabs";

export function useEntityLink(
  space: string,
  record: Item | null,
  setRecord: Dispatch<SetStateAction<Item | null>>,
  setDraft: Dispatch<SetStateAction<Item | null>>,
  setTab: (tab: string) => void,
) {
  // An ?entity= link opens that entity once it has loaded, on its ?tab= tab
  // when one is named (the Space access matrix opens Access).
  const [[linked, linkedTab]] = useState(() => {
      const q = new URLSearchParams(location.search);
      return [q.get("entity"), q.get("tab")];
    }),
    linkedQuery = useApiQuery<Item>(linked ? "/v1/entities/" + linked : null, {
      refetchInterval: false,
    }),
    [linkHandled, setLinkHandled] = useState(!linked);
  if (!linkHandled && (linkedQuery.data || linkedQuery.error)) {
    setLinkHandled(true);
    if (!record && linkedQuery.data?.space_id === space) {
      setRecord(linkedQuery.data);
      setDraft(definitionOf(linkedQuery.data.draft_json));
      if (entityTabs.some(([id]) => id === linkedTab)) setTab(linkedTab!);
    }
  }
  return { linkedQuery, linkHandled };
}
