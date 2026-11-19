import { text, type EntityCatalog, type Item } from "../api-types";
import { Button, NavList, NavListNote } from "../ui";

// The Space's entity list beside the designer: each entity with a dot for
// unpublished changes, the open one selected, and the New entity action.
export function EntityNav({
  spaceName,
  items,
  record,
  draft,
  dirty,
  catalog,
  busy,
  open,
  create,
}: {
  spaceName: string;
  items: Item[];
  record: Item | null;
  draft: Item | null;
  dirty: boolean;
  catalog: EntityCatalog;
  busy: boolean;
  open: (id: string) => Promise<void>;
  create: () => void;
}) {
  return (
    <NavList
      label={spaceName ? spaceName + " entities" : "Entities"}
      items={items.map((v) => ({
        id: text(v.id),
        label: text(v.id === record?.id ? draft?.label : v.label),
        dot: !!v.has_draft_changes || (v.id === record?.id && dirty),
        dotLabel: ", unpublished changes",
      }))}
      value={text(record?.id)}
      onPick={(id) => {
        if (id !== record?.id) void open(id);
      }}
      disabled={busy}
      footer={
        <>
          <Button onClick={create} disabled={busy}>
            + New entity
          </Button>
          {catalog.templates.length > 0 && (
            <NavListNote>
              Templates:{" "}
              {catalog.templates.map((t: Item) => text(t.label)).join(", ")}
            </NavListNote>
          )}
        </>
      }
    />
  );
}
