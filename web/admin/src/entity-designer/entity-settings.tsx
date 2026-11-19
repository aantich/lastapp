import { api } from "../api";
import {
  Button,
  Card,
  CardBody,
  Checkbox,
  Columns,
  Disclosure,
  FormField,
  Inline,
  Select,
  Stack,
  TextField,
  confirmAction,
} from "../ui";
import { items, num, text, type Item } from "../api-types";
import { FieldChoices } from "./field-choices";
import { inputValue, strings, type Designer } from "./shared";

const fieldOptions = (fields: Item[], type: string) => [
  { value: "", label: "None" },
  ...fields
    .filter((f) => f.type === type)
    .map((f) => ({ value: text(f.key), label: text(f.label) })),
];

// Entity-wide settings: name, ownership, default fields, sharing and the
// lifecycle (archive/restore).
export function EntitySettings({ d }: { d: Designer }) {
  const { busy, dirty, draft, record, run, save, setDraft, setRecord } = d;
  const fields = items(draft.fields),
    archived = record?.state === "archived";
  return (
    <Card>
      <CardBody>
        <Disclosure
          summary="Entity settings"
          meta="Name, ownership, default fields and sharing"
        >
          <Stack gap="md">
            <Columns min="sm" gap="md">
              <TextField
                label="Name"
                value={inputValue(draft.label)}
                onChange={(e) => setDraft({ ...draft, label: e.target.value })}
              />
              <FormField label="Owner field">
                <Select
                  value={text(draft.owner_field)}
                  onChange={(owner_field) =>
                    setDraft({ ...draft, owner_field })
                  }
                  options={fieldOptions(fields, "user")}
                />
              </FormField>
              <FormField label="Owning team field">
                <Select
                  value={text(draft.team_field)}
                  onChange={(team_field) => setDraft({ ...draft, team_field })}
                  options={fieldOptions(fields, "team")}
                />
              </FormField>
            </Columns>
            <FieldChoices
              legend="Default fields"
              description="Included when an App doesn't select specific fields."
              fields={fields}
              value={strings(draft.default_projection)}
              onChange={(v) => setDraft({ ...draft, default_projection: v })}
              empty="Add fields to choose the defaults."
            />
            <Checkbox
              label="Allow record sharing"
              checked={!!draft.sharing}
              onChange={(sharing) => setDraft({ ...draft, sharing })}
            />
            <Inline gap="md">
              <Button
                variant="danger"
                disabled={busy || !!record?.pending_version_id}
                onClick={async () => {
                  if (
                    await confirmAction(
                      archived
                        ? {
                            title: "Restore entity?",
                            message: "Restore this entity?",
                            confirmLabel: "Restore",
                          }
                        : {
                            title: "Archive entity?",
                            message:
                              "Archive this entity? Its API will stop serving requests; data is preserved.",
                            confirmLabel: "Archive",
                            danger: true,
                          },
                    )
                  )
                    void run(async () => {
                      const saved = dirty ? await save() : record!;
                      const v = await api<Item>(
                        "/v1/entities/" + saved.id,
                        "PATCH",
                        {
                          definition: draft,
                          state: archived ? "active" : "archived",
                        },
                        num(saved.revision),
                      );
                      setRecord(v);
                    });
                }}
              >
                {archived ? "Restore entity" : "Archive entity"}
              </Button>
            </Inline>
          </Stack>
        </Disclosure>
      </CardBody>
    </Card>
  );
}
