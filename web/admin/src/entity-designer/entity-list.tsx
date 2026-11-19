import { useRef, useState } from "react";
import { api } from "../api";
import { FormField, Select, TextField } from "../ui";
import { text, type EntityCatalog, type Item } from "../api-types";
import { EditorDialog, EditorForm } from "./editor-dialog";
import { emptyDefinition, keyOf } from "./shared";

// "+ New entity": a blank entity or one of the catalog's templates.
// Templates start without access grants.
export function NewEntityDialog({
  space,
  catalog,
  run,
  close,
  opened,
}: {
  space: string;
  catalog: EntityCatalog;
  run: (fn: () => Promise<unknown>) => Promise<void>;
  close: () => void;
  opened: (record: Item) => void;
}) {
  const [name, setName] = useState(""),
    [template, setTemplate] = useState("");
  const creationID = useRef(crypto.randomUUID());
  return (
    <EditorDialog
      title="New entity"
      description="Describe your data once. Every App in this Space shares its API, access rules and validation. Templates start without access grants."
      close={close}
    >
      <EditorForm
        submitLabel="Create entity"
        submit={() => {
          if (!name.trim()) return;
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
            creationID.current = crypto.randomUUID();
            opened(v);
            close();
          });
        }}
      >
        <FormField label="Start from">
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
        </FormField>
        <TextField
          label="Entity name"
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="For example, Deal"
        />
      </EditorForm>
    </EditorDialog>
  );
}
