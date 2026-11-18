import { useState, type ReactNode } from "react";
import {
  ActivityList,
  Alert,
  Button,
  Card,
  CardBody,
  CardFooter,
  EmptyState,
  Inline,
  PageHeader,
  SegmentedControl,
  Select,
  Skeleton,
  Stack,
} from "./ui";
import {
  text,
  type AgentConfig,
  type AgentGrant,
  type Page,
} from "./api-types";
import { useApiQuery } from "./query";
import { describeEvents } from "./activity";
import { agentConfigPath } from "./connect-agent";
import { auditActors } from "./actors";

// One audit feed for Platform and Tenant activity: people, agents acting
// for people and the background worker, newest first, one row per event (or
// per run of one actor's changes to one resource). The actor, kind and
// resource filters run on the server before search and paging; the kinds
// are those the trail holds (…/audit-kinds) and the URL may preset the actor
// (?actor=) and resource (?resource_id=). `grants` names agents where the
// screen may read them. With `title` the feed draws the page heading with
// its filters beside `actions`; otherwise the filters sit above the list.
export function ActivityLog({
  path,
  grants,
  title,
  actions,
}: {
  path: string;
  grants?: string;
  title?: ReactNode;
  actions?: ReactNode;
}) {
  const [linked] = useState(() => new URLSearchParams(location.search));
  const [actor, setActor] = useState(() => linked.get("actor") || ""),
    [kind, setKind] = useState(""),
    [resource, setResource] = useState(() => linked.get("resource_id") || ""),
    [page, setPage] = useState(1);
  const kinds =
    useApiQuery<{ items: string[] }>(
      path.replace(/audit-events$/, "audit-kinds"),
    ).data?.items || [];
  const named = useApiQuery<Page<AgentGrant>>(grants || null).data?.items || [];
  const presets =
    useApiQuery<AgentConfig>(grants ? agentConfigPath : null).data?.presets ||
    [];
  const params = new URLSearchParams({ page: String(page) });
  if (actor) params.set("actor", actor);
  if (kind) params.set("kind", kind);
  if (resource) params.set("resource_id", resource);
  const result = useApiQuery<Page>(path + "?" + params.toString());
  const rows = result.data?.items || [],
    more = !!result.data?.pagination?.has_more;
  const filter = (set: (value: string) => void) => (value: string) => {
    set(value);
    setPage(1);
  };
  const resourceName =
    text(rows.find((r) => r.resource_name)?.resource_name) || "one resource";
  const filters = (
    <>
      {kinds.length > 0 && (
        <Select
          aria-label="Kind"
          value={kind}
          onChange={filter(setKind)}
          options={[
            { value: "", label: "All kinds" },
            ...kinds.map((k) => ({ value: k, label: k.replace(/_/g, " ") })),
          ]}
        />
      )}
      <SegmentedControl
        label="Filter actors"
        options={auditActors}
        value={actor}
        onChange={filter(setActor)}
      />
    </>
  );
  const scope = resource && (
    <>
      Changes to {resourceName}.{" "}
      <Button variant="link" onClick={() => filter(setResource)("")}>
        Show all activity
      </Button>
    </>
  );
  const list = (
    <Card aria-busy={result.isPending || undefined}>
      {result.isPending ? (
        <CardBody>
          <Stack gap="md">
            <Skeleton width="70%" />
            <Skeleton width="55%" />
            <Skeleton width="62%" />
          </Stack>
        </CardBody>
      ) : rows.length ? (
        <ActivityList
          label="Activity"
          variant="rows"
          items={describeEvents(rows, named, [], {
            presets,
            // Within one resource's trail every change is its own row.
            changes: resource
              ? undefined
              : (id, count) => (
                  <Button
                    variant="link"
                    onClick={() => filter(setResource)(id)}
                  >
                    {count} changes
                  </Button>
                ),
          })}
        />
      ) : (
        <EmptyState>
          {actor || kind || resource
            ? "No activity matches these filters."
            : "No activity yet."}
        </EmptyState>
      )}
      {(page > 1 || more) && (
        <CardFooter>
          <Button
            size="sm"
            aria-label="Newer activity"
            disabled={result.isPending || page === 1}
            onClick={() => setPage((p) => p - 1)}
          >
            Newer
          </Button>
          <Button
            size="sm"
            aria-label="Older activity"
            disabled={result.isPending || !more}
            onClick={() => setPage((p) => p + 1)}
          >
            Older
          </Button>
        </CardFooter>
      )}
    </Card>
  );
  const error = result.error && <Alert>{result.error.message}</Alert>;
  return title !== undefined ? (
    <>
      <PageHeader
        title={title}
        description={scope}
        actions={
          // The actor filter ends the row, as in the mockup.
          <>
            {actions}
            {filters}
          </>
        }
      />
      {error}
      {list}
    </>
  ) : (
    <Stack>
      <Inline gap="md">
        {filters}
        {scope}
      </Inline>
      {error}
      {list}
    </Stack>
  );
}
