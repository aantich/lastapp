import { Collection } from "./collection";

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
              <strong>{r.action}</strong>
              <small>{r.resource_type}</small>
            </>
          ),
        ],
        ["Actor", (r) => r.actor_name],
        ["Time", (r) => new Date(r.created_at).toLocaleString()],
      ]}
    />
  );
}
