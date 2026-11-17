import { useRef, useState } from "react";
import { api } from "../api";
import { Select } from "../ui";
import { text, type EntityCatalog, type Item } from "../api-types";
import { emptyDefinition, keyOf } from "./shared";

// The Space's entities and the form that creates one.
export function EntityList({
  space,
  catalog,
  items,
  busy,
  run,
  open,
  opened,
}: {
  space: string;
  catalog: EntityCatalog;
  items: Item[];
  busy: boolean;
  run: (fn: () => Promise<unknown>) => Promise<void>;
  open: (id: string) => void;
  opened: (record: Item) => void;
}) {
  const [name, setName] = useState(""),
    [template, setTemplate] = useState("");
  const creationID = useRef(crypto.randomUUID());
  return (
    <>
      <div className="entity-intro">
        <h2>Business entities</h2>
        <p>
          Describe your data once. Every App in this Space shares its API,
          access rules and validation.
        </p>
      </div>
      <form
        className="entity-create panel"
        onSubmit={(e) => {
          e.preventDefault();
          void run(async () => {
            const v = await api<Item>(
              "/v1/spaces/" + space + "/entities",
              "POST",
              {
                id: creationID.current,
                definition: template
                  ? {
                      ...structuredClone(
                        catalog.templates.find((d: Item) => d.key === template),
                      ),
                      key: keyOf(name),
                      label: name,
                    }
                  : emptyDefinition(name, catalog.format_version),
              },
            );
            opened(v);
            setName("");
            creationID.current = crypto.randomUUID();
            history.replaceState(
              null,
              "",
              location.pathname + "?entity=" + encodeURIComponent(text(v.id)),
            );
          });
        }}
      >
        <label className="field">
          <span>Start from</span>
          <Select
            value={template}
            onChange={setTemplate}
            options={[
              { value: "", label: "Blank entity" },
              ...(catalog.templates || []).map((d: Item) => ({
                value: text(d.key),
                label: text(d.label),
              })),
            ]}
          />
        </label>
        <label className="field">
          <span>Entity name</span>
          <input
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="For example, Deal"
          />
        </label>
        <button className="primary" disabled={busy || !name.trim()}>
          Create entity
        </button>
      </form>
      <div className="entity-list">
        {items.map((v) => (
          <button
            key={text(v.id)}
            disabled={busy}
            className="panel entity-card"
            onClick={() => open(text(v.id))}
          >
            <strong>{text(v.label)}</strong>
            <span>
              {v.pending_version_id
                ? "Publishing"
                : v.state === "archived"
                  ? "Archived"
                  : v.active_version_id
                    ? "Published"
                    : "Draft"}
            </span>
            <small>{text(v.entity_key)}</small>
          </button>
        ))}
      </div>
    </>
  );
}
