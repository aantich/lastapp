import { Collection } from "./ui";
import { text } from "./api-types";

// One audit-event table for Platform and Tenant activity.
export function ActivityLog({ path }: { path: string }) {
  return (
    <Collection
      path={path}
      columns={[
        [
          "Event",
          (r) => (
            <>
              <strong>{text(r.action)}</strong>
              <small>{text(r.resource_type)}</small>
            </>
          ),
        ],
        ["Actor", (r) => text(r.actor_name)],
        ["Time", (r) => new Date(text(r.created_at)).toLocaleString()],
      ]}
    />
  );
}
