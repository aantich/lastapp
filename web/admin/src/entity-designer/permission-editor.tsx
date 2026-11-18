import { useState } from "react";
import {
  Checkbox,
  Columns,
  FieldGroup,
  FormField,
  RemoteSelect,
  Select,
  TextField,
} from "../ui";
import { items, text, type EntityCatalog, type Item } from "../api-types";
import { Expression, FieldChoices } from "./expression";
import { EditorForm } from "./editor-dialog";
import {
  inputValue,
  isExpression,
  keyOf,
  optionalText,
  relationshipLabels,
  strings,
} from "./shared";

// Adds or edits one permission path: who, which operation, and the fields
// and records it reaches. Fields and their record condition are one unit.
export function PermissionEditor({
  value,
  definition,
  roles,
  catalog,
  tenant,
  save,
}: {
  value: Item;
  definition: Item;
  roles: Item[];
  catalog: EntityCatalog;
  tenant: string;
  save: (v: Item) => void;
}) {
  const [p, set] = useState({ ...value });
  const patch = (v: Item) => set({ ...p, ...v });
  const fields = items(definition.fields);
  const withFields = ["read", "create", "update"].includes(text(p.operation));
  return (
    <EditorForm submitLabel="Save access rule" submit={() => save(p)}>
      <TextField
        label="Name"
        required
        data-autofocus
        value={inputValue(p.label)}
        onChange={(e) =>
          patch({
            label: e.target.value,
            ...(!value.key ? { key: keyOf(e.target.value) } : {}),
          })
        }
      />
      <Columns min={220} gap="md">
        <FormField label="Who">
          <Select
            value={p.roles ? "roles" : optionalText(p.relationship)}
            onChange={(who) =>
              patch(
                who === "roles"
                  ? { roles: [], relationship: undefined }
                  : { roles: undefined, relationship: who },
              )
            }
            options={[
              { value: "roles", label: "Space roles" },
              ...catalog.relationships.map((t: string) => ({
                value: t,
                label: relationshipLabels[t] || t,
              })),
            ]}
          />
        </FormField>
        <FormField label="Can">
          <Select
            value={optionalText(p.operation)}
            onChange={(operation) => patch({ operation })}
            options={catalog.operations.map((op: string) => ({
              value: op,
              label:
                op === "acl.manage" ? "Manage record access (advanced)" : op,
            }))}
          />
        </FormField>
      </Columns>
      {!!p.roles && (
        <FieldChoices
          legend="Which Space roles"
          fields={roles.map((r) => ({ key: r.id, label: r.display_name }))}
          value={strings(p.roles)}
          onChange={(v) => patch({ roles: v })}
          empty="Create a business role in Space → Access → Manage roles."
        />
      )}
      {["user", "team"].includes(text(p.relationship)) && (
        <FormField label="Recipient">
          <RemoteSelect
            initial={[]}
            path={
              "/v1/tenants/" +
              tenant +
              (p.relationship === "user" ? "/recipients" : "/teams")
            }
            label="Recipient"
            value={text(p.subject)}
            onChange={(v) => patch({ subject: v })}
          />
        </FormField>
      )}
      <FieldGroup
        legend="Path"
        description={
          withFields
            ? "The fields this rule reaches and the records they are on. New fields never join this list automatically."
            : "The records this rule reaches."
        }
        framed
      >
        {withFields && (
          <FieldChoices
            legend="Which fields"
            fields={fields.filter(
              (f) => p.operation === "read" || !f.calculation,
            )}
            value={strings(p.fields)}
            onChange={(v) => patch({ fields: v })}
            empty="Add fields to this entity first."
          />
        )}
        <Checkbox
          label="Limit to records matching a condition"
          checked={!!p.when}
          onChange={(on) =>
            patch({
              when: on
                ? {
                    op: "eq",
                    args: [{ field: fields[0]?.key || "" }, { literal: "" }],
                  }
                : undefined,
            })
          }
        />
        {isExpression(p.when) && (
          <Expression
            value={p.when}
            onChange={(v) => patch({ when: v })}
            fields={fields}
            catalog={catalog}
          />
        )}
        {p.operation === "update" && (
          <>
            <Checkbox
              label="Authorize a particular change"
              checked={!!p.change_when}
              onChange={(on) =>
                patch({
                  change_when: on
                    ? {
                        op: "eq",
                        args: [
                          { field: "old." + (fields[0]?.key || "") },
                          { literal: "" },
                        ],
                      }
                    : undefined,
                })
              }
            />
            {isExpression(p.change_when) && (
              <Expression
                value={p.change_when}
                onChange={(v) => patch({ change_when: v })}
                fields={fields}
                catalog={catalog}
                label="Change"
                transition
              />
            )}
          </>
        )}
      </FieldGroup>
      {p.operation === "acl.manage" && (
        <FieldChoices
          legend="Permissions this user can share"
          fields={items(definition.permissions)
            .filter(
              (v) =>
                Array.isArray(v.roles) &&
                v.roles.length > 0 &&
                !["create", "acl.manage"].includes(text(v.operation)),
            )
            .map((v) => ({ key: v.key, label: v.label }))}
          value={strings(p.delegable)}
          onChange={(v) => patch({ delegable: v })}
          empty="Only role-based read, update or delete rules can be shared."
        />
      )}
    </EditorForm>
  );
}
