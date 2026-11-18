import { useState } from "react";
import {
  ActivityList,
  CardHeader,
  Collection,
  SegmentedControl,
  Select,
} from "./ui";
import type { AgentGrant, Page } from "./api-types";
import { useApiQuery } from "./query";
import { describeEvents } from "./activity";

// The actor filter's choices, the server's actor kinds (AuditFilter.Actor).
const actors = [
  { value: "", label: "All" },
  { value: "person", label: "People" },
  { value: "agent", label: "Agents" },
  { value: "system", label: "System" },
];

// One audit feed for Platform and Tenant activity: people, agents acting
// for people and the background worker, newest first. The actor and kind
// filters run on the server before search and paging; the kinds are those
// the trail holds (…/audit-kinds). `grants` names agents where the screen
// may read them.
export function ActivityLog({
  path,
  grants,
}: {
  path: string;
  grants?: string;
}) {
  const [actor, setActor] = useState(""),
    [kind, setKind] = useState("");
  const kinds =
    useApiQuery<{ items: string[] }>(
      path.replace(/audit-events$/, "audit-kinds"),
    ).data?.items || [];
  const named = useApiQuery<Page<AgentGrant>>(grants || null).data?.items || [];
  const params = new URLSearchParams();
  if (actor) params.set("actor", actor);
  if (kind) params.set("kind", kind);
  return (
    <Collection
      path={path}
      caption="Activity"
      columns={[]}
      header={
        <CardHeader
          title="Audit trail"
          description="Who changed what: people, agents acting for people and the system worker."
        />
      }
      params={params.toString()}
      filters={
        <>
          <SegmentedControl
            label="Actor"
            options={actors}
            value={actor}
            onChange={setActor}
          />
          <Select
            aria-label="Kind"
            value={kind}
            onChange={setKind}
            options={[
              { value: "", label: "All kinds" },
              ...kinds.map((k) => ({ value: k, label: k.replace(/_/g, " ") })),
            ]}
          />
        </>
      }
      empty={
        actor || kind
          ? "No activity matches these filters."
          : "No activity yet."
      }
      render={(rows) => (
        <ActivityList
          label="Activity"
          variant="rows"
          items={describeEvents(rows, named)}
        />
      )}
    />
  );
}
