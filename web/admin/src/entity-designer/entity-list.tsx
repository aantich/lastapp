import { useRef, useState } from "react";
import { Database, Plus } from "lucide-react";
import { api } from "../api";
import {
  Badge,
  Button,
  Card,
  CardBody,
  CardHeader,
  Columns,
  DataTable,
  EmptyState,
  FormField,
  Inline,
  Select,
  Stack,
  StatusBadge,
  TextField,
} from "../ui";
import { text, type EntityCatalog, type Item } from "../api-types";
import { emptyDefinition, entityState, keyOf } from "./shared";

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
    <Stack>
      <Card>
        <CardHeader
          title="Business entities"
          description="Describe your data once. Every App in this Space shares its API, access rules and validation."
        />
        <CardBody>
          {items.length ? (
            <DataTable
              caption="Business entities"
              rows={items}
              rowKey={(v) => text(v.id)}
              columns={[
                {
                  key: "name",
                  header: "Entity",
                  render: (v) => (
                    <Button
                      variant="link"
                      disabled={busy}
                      onClick={() => open(text(v.id))}
                    >
                      {text(v.label)}
                    </Button>
                  ),
                },
                {
                  key: "key",
                  header: "API key",
                  render: (v) => <Badge mono>{text(v.entity_key)}</Badge>,
                },
                {
                  key: "state",
                  header: "State",
                  render: (v) => <StatusBadge state={entityState(v)} />,
                },
              ]}
            />
          ) : (
            <EmptyState
              compact
              icon={<Database size={28} aria-hidden />}
              title="No entities yet"
            >
              Start from a template or a blank entity below.
            </EmptyState>
          )}
        </CardBody>
      </Card>
      <Card>
        <CardHeader
          title="New entity"
          level={3}
          description="Templates start without access grants."
        />
        <CardBody>
          <form
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
                            catalog.templates.find(
                              (d: Item) => d.key === template,
                            ),
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
                  location.pathname +
                    "?entity=" +
                    encodeURIComponent(text(v.id)),
                );
              });
            }}
          >
            <Stack gap="md">
              <Columns min={220} gap="md">
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
              </Columns>
              <Inline>
                <Button
                  type="submit"
                  variant="primary"
                  icon={<Plus size={15} aria-hidden />}
                  disabled={busy || !name.trim()}
                >
                  Create entity
                </Button>
              </Inline>
            </Stack>
          </form>
        </CardBody>
      </Card>
    </Stack>
  );
}
