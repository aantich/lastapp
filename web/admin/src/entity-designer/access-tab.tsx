import { Plus } from "lucide-react";
import { Button, EmptyState, Panel, Icon } from "../ui";
import { items } from "../api-types";
import { PathCards, UnreachedFields } from "./path-cards";
import { type Designer } from "./shared";

// The Fields tab's side panel: the Access tab's permission paths, read
// only, with the fields no read path reaches.
export function PathsSummary({ d }: { d: Designer }) {
  return (
    <Panel
      title="Permission paths"
      lead="Who reaches which fields, and on which records."
    >
      {items(d.draft.permissions).length ? (
        <PathCards d={d} />
      ) : (
        <EmptyState compact title="No access yet">
          Access is denied until a path matches. Add one on the Access tab.
        </EmptyState>
      )}
      <UnreachedFields d={d} />
    </Panel>
  );
}

// The Access tab: permission paths, each one unit of who, which operation,
// which fields and which records. Everything else is denied.
export function AccessTab({ d }: { d: Designer }) {
  const { draft, setEditPermission } = d;
  return (
    <Panel
      title="Permission paths"
      lead="Fields and their record predicates stay together. Apps can only narrow them. Administrative roles grant no data access."
      actions={
        <Button
          size="sm"
          icon={<Icon icon={Plus} size="md" aria-hidden />}
          onClick={() =>
            setEditPermission({
              key: "",
              label: "",
              operation: "read",
              relationship: "member",
              fields: [],
            })
          }
        >
          Add access rule
        </Button>
      }
    >
      {items(draft.permissions).length ? (
        <PathCards d={d} editable />
      ) : (
        <EmptyState compact title="No access yet">
          Add a rule by choosing who, what they can do, and which fields. Access
          is denied until a path matches.
        </EmptyState>
      )}
      <UnreachedFields d={d} />
    </Panel>
  );
}
