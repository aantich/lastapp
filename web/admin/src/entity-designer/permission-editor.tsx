import { useState } from "react";
import { RemoteSelect } from "../remote-select";
import { Select } from "../select";
import { items, text, type EntityCatalog, type Item } from "../api-types";
import { Expression, FieldChoices } from "./expression";
import {
  inputValue,
  isExpression,
  keyOf,
  optionalText,
  strings,
} from "./shared";

// Adds or edits one access rule: who, which operation, which fields and when.
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
  const fields = items(definition.fields),
    roleIDs = strings(p.roles);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        save(p);
      }}
    >
      <label className="field">
        <span>Name</span>
        <input
          autoFocus
          required
          value={inputValue(p.label)}
          onChange={(e) =>
            patch({
              label: e.target.value,
              ...(!value.key ? { key: keyOf(e.target.value) } : {}),
            })
          }
        />
      </label>
      <label className="field">
        <span>Who</span>
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
              label:
                (
                  {
                    member: "All active Tenant members",
                    owner: "Record owner",
                    team_member: "Members of record's team",
                    team_reviewer: "Reviewers of record's team",
                    team_admin: "Admins of record's team",
                    user: "Specific user",
                    team: "Specific team",
                  } as Record<string, string>
                )[t] || t,
            })),
          ]}
        />
      </label>
      {!!p.roles && (
        <div className="field-choices">
          {roles.map((role) => (
            <label key={text(role.id)}>
              <input
                type="checkbox"
                checked={roleIDs.includes(text(role.id))}
                onChange={(e) =>
                  patch({
                    roles: e.target.checked
                      ? [...roleIDs, role.id]
                      : roleIDs.filter((v) => v !== role.id),
                  })
                }
              />
              {text(role.display_name)}
            </label>
          ))}
          {!roles.length && (
            <p>Create a business role in Space → Access → Manage roles.</p>
          )}
        </div>
      )}
      {["user", "team"].includes(text(p.relationship)) && (
        <label className="field">
          <span>Recipient</span>
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
        </label>
      )}
      <label className="field">
        <span>Can</span>
        <Select
          value={optionalText(p.operation)}
          onChange={(operation) => patch({ operation })}
          options={catalog.operations.map((op: string) => ({
            value: op,
            label: op === "acl.manage" ? "Manage record access (advanced)" : op,
          }))}
        />
      </label>
      {["read", "create", "update"].includes(text(p.operation)) && (
        <>
          <p>Which fields</p>
          <FieldChoices
            fields={fields.filter(
              (f) => p.operation === "read" || !f.calculation,
            )}
            value={strings(p.fields)}
            onChange={(v) => patch({ fields: v })}
          />
        </>
      )}
      <label>
        <input
          type="checkbox"
          checked={!!p.when}
          onChange={(e) =>
            patch({
              when: e.target.checked
                ? {
                    op: "eq",
                    args: [{ field: fields[0]?.key || "" }, { literal: "" }],
                  }
                : undefined,
            })
          }
        />{" "}
        Limit to records matching a condition
      </label>
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
          <label>
            <input
              type="checkbox"
              checked={!!p.change_when}
              onChange={(e) =>
                patch({
                  change_when: e.target.checked
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
            />{" "}
            Authorize a particular change
          </label>
          {isExpression(p.change_when) && (
            <Expression
              value={p.change_when}
              onChange={(v) => patch({ change_when: v })}
              fields={fields}
              catalog={catalog}
              transition
            />
          )}
        </>
      )}
      {p.operation === "acl.manage" && (
        <>
          <p>Permissions this user can share</p>
          <FieldChoices
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
          />
        </>
      )}
      <button className="primary">Save access rule</button>
    </form>
  );
}
