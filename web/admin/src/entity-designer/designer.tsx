import { useEffect, useState, useRef } from "react";
import { ArrowLeft, Pencil, Plus, Trash2, X } from "lucide-react";
import { api } from "../api";
import { apiKey, queryClient, useApiQuery } from "../query";
import { RemoteSelect } from "../remote-select";
import { Select } from "../select";
import { NumberInput } from "../number-input";
import type { EntityCatalog, Item, Page } from "../api-types";
import {
  Expression,
  FieldChoices,
  ValueInputs,
  choiceLabel,
} from "./expression";
import "./entity.css";
import { useAction } from "../use-action";
import { confirmAction } from "../confirm";
import { beforeNavigateEvent, type BeforeNavigateEvent } from "../console-url";
const keyOf = (name: string) => {
  const bare = name
    .normalize("NFKD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return (
    bare
      ? /^[a-z]/.test(bare)
        ? bare
        : "item_" + bare
      : "item_" + crypto.randomUUID().replaceAll("-", "")
  ).slice(0, 48);
};
const fieldLabel = (key: string) => key[0].toUpperCase() + key.slice(1);
const discardChanges = () =>
  confirmAction({
    title: "Discard unsaved changes?",
    message: "Your unsaved entity changes will be lost.",
    confirmLabel: "Discard changes",
    danger: true,
  });
const emptyDefinition = (name: string, format: number) => ({
  format_version: format,
  key: keyOf(name),
  label: name,
  fields: [],
  rules: [],
  permissions: [],
  default_projection: [],
});
export function EntityDesigner({
  space,
  tenant,
  me,
}: {
  space: string;
  tenant: string;
  me: Item;
}) {
  const [record, setRecord] = useState<Item | null>(null),
    [draft, setDraft] = useState<Item | null>(null),
    [tab, setTab] = useState("fields"),
    [error, setError] = useState(""),
    [name, setName] = useState(""),
    [template, setTemplate] = useState(""),
    [review, setReview] = useState(false);
  const [editField, setEditField] = useState<Item | null>(null),
    [editPermission, setEditPermission] = useState<Item | null>(null),
    [editRule, setEditRule] = useState<Item | null>(null);
  const creationID = useRef(crypto.randomUUID());
  const dirty =
    !!record && JSON.stringify(draft) !== JSON.stringify(record.draft_json);
  const listPath = "/v1/spaces/" + space + "/entities";
  // The catalog and role list do not change while the designer is open.
  const catalogQuery = useApiQuery<EntityCatalog>("/v1/entity-capabilities", {
      refetchInterval: false,
    }),
    rolesQuery = useApiQuery<Page>(
      "/v1/tenants/" + tenant + "/spaces/" + space + "/roles",
      { refetchInterval: false },
    ),
    listQuery = useApiQuery<Page>(listPath);
  const catalog = catalogQuery.data,
    roles = (rolesQuery.data?.items || []).filter((x) => x.domain === "data"),
    items = listQuery.data?.items || [];
  const load = () =>
    queryClient.invalidateQueries({ queryKey: apiKey(listPath) });
  // An ?entity= link opens that entity once it has loaded.
  const [linked] = useState(() =>
      new URLSearchParams(location.search).get("entity"),
    ),
    linkedQuery = useApiQuery<Item>(linked ? "/v1/entities/" + linked : null, {
      refetchInterval: false,
    }),
    [linkHandled, setLinkHandled] = useState(false);
  if (!linkHandled && linkedQuery.data) {
    setLinkHandled(true);
    if (!record && linkedQuery.data.space_id === space) {
      setRecord(linkedQuery.data);
      setDraft(linkedQuery.data.draft_json);
    }
  }
  // While a publication is pending, poll the entity every 1.5 s until the
  // pending version clears. refetchInterval never overlaps requests. Only
  // responses fetched for this publication count, never an older cached read.
  const pendingQuery = useApiQuery<Item>(
      record?.pending_version_id ? "/v1/entities/" + record.id : null,
      {
        refetchInterval: (q) =>
          q.state.data && !q.state.data.pending_version_id ? false : 1500,
      },
    ),
    [polled, setPolled] = useState(0);
  if (
    pendingQuery.data &&
    pendingQuery.isFetchedAfterMount &&
    pendingQuery.dataUpdatedAt !== polled
  ) {
    const v = pendingQuery.data;
    setPolled(pendingQuery.dataUpdatedAt);
    // Keep unsaved edits: take only publication state from the server.
    setRecord((current) =>
      dirty &&
      current &&
      JSON.stringify(current.draft_json) !== JSON.stringify(v.draft_json)
        ? {
            ...current,
            publications: v.publications,
            pending_version_id: v.pending_version_id,
            active_version_id: v.active_version_id,
            active_definition: v.active_definition,
          }
        : v,
    );
    if (!dirty) setDraft(v.draft_json);
    if (!v.pending_version_id) void load();
  }
  const readError =
    catalogQuery.error ||
    rolesQuery.error ||
    listQuery.error ||
    linkedQuery.error ||
    pendingQuery.error;
  useEffect(() => {
    if (!dirty) return;
    const fn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    const navigate = (e: Event) => {
      e.preventDefault();
      void discardChanges().then((discard) => {
        if (discard) (e as BeforeNavigateEvent).detail.proceed();
      });
    };
    window.addEventListener("beforeunload", fn);
    window.addEventListener(beforeNavigateEvent, navigate);
    return () => {
      window.removeEventListener("beforeunload", fn);
      window.removeEventListener(beforeNavigateEvent, navigate);
    };
  }, [dirty]);
  // Designer actions share the console's mutation guard: one at a time, the
  // previous error cleared, cached reads (the entity list) refreshed after.
  const { busy, run } = useAction(space, {
    onSuccess: () => {},
    onError: setError,
  });
  async function open(id: string) {
    if (dirty && !(await discardChanges())) return;
    await run(async () => {
      const v = await api<Item>("/v1/entities/" + id);
      setRecord(v);
      setDraft(v.draft_json);
      history.replaceState(
        null,
        "",
        location.pathname + "?entity=" + encodeURIComponent(v.id),
      );
      setReview(false);
      setTab("fields");
    });
  }
  async function save() {
    const v = await api<Item>(
      "/v1/entities/" + record!.id,
      "PATCH",
      { definition: draft },
      record!.revision,
    );
    setRecord(v);
    setDraft(v.draft_json);
    return v;
  }
  const replace = (list: string, key: string, v: Item) =>
    setDraft((d) => ({
      ...d,
      ...(list === "fields" && d!.fields.length === 0
        ? { default_projection: [v.key] }
        : {}),
      [list]: d![list].some((x: Item) => x.key === key)
        ? d![list].map((x: Item) => (x.key === key ? v : x))
        : [...d![list], v],
    }));
  const remove = async (list: string, key: string) => {
    const kind = list === "fields" ? "field" : "definition";
    if (
      await confirmAction({
        title: "Remove " + kind + "?",
        message: "Remove this " + kind + " from the draft?",
        confirmLabel: "Remove",
        danger: true,
      })
    )
      setDraft((d) => ({
        ...d,
        [list]: d![list].filter((x: Item) => x.key !== key),
      }));
  };
  const shownError = error || readError?.message || "";
  if (!catalog)
    return (
      <p role={shownError ? "alert" : undefined}>
        {shownError || "Loading entity designer…"}
      </p>
    );
  return (
    <section className="entity-designer">
      {shownError && (
        <div className="error" role="alert">
          {shownError}
        </div>
      )}
      {!draft ? (
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
                setRecord(v);
                setDraft(v.draft_json);
                setName("");
                creationID.current = crypto.randomUUID();
                history.replaceState(
                  null,
                  "",
                  location.pathname + "?entity=" + encodeURIComponent(v.id),
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
                    value: d.key,
                    label: d.label,
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
                key={v.id}
                disabled={busy}
                className="panel entity-card"
                onClick={() => void open(v.id)}
              >
                <strong>{v.label}</strong>
                <span>
                  {v.pending_version_id
                    ? "Publishing"
                    : v.state === "archived"
                      ? "Archived"
                      : v.active_version_id
                        ? "Published"
                        : "Draft"}
                </span>
                <small>{v.entity_key}</small>
              </button>
            ))}
          </div>
        </>
      ) : (
        <>
          <header className="entity-heading">
            <div className="entity-title">
              <button
                className="text-button"
                onClick={async () => {
                  if (!dirty || (await discardChanges())) {
                    setDraft(null);
                    setRecord(null);
                    history.replaceState(null, "", location.pathname);
                  }
                }}
              >
                <ArrowLeft size={14} /> Entities
              </button>
              <div className="entity-title-copy">
                <h2>{draft.label}</h2>
                <small>
                  {dirty
                    ? "Unsaved changes"
                    : record?.pending_version_id
                      ? "Publishing…"
                      : record?.active_version_id
                        ? "Published · draft saved"
                        : "Draft saved"}
                </small>
              </div>
            </div>
            <div className="entity-heading-actions">
              <button
                className="secondary"
                disabled={!dirty || busy}
                onClick={() =>
                  void run(async () => {
                    await save();
                  })
                }
              >
                Save draft
              </button>
              <button
                className="primary"
                disabled={
                  busy ||
                  !!record?.pending_version_id ||
                  record?.state === "archived"
                }
                onClick={() => setReview(true)}
              >
                Review changes
              </button>
            </div>
          </header>
          <nav className="tabs entity-tabs" aria-label="Entity designer">
            {["fields", "access", "rules", "test"].map((t) => (
              <button
                key={t}
                className={tab === t ? "active" : ""}
                onClick={() => setTab(t)}
              >
                {t[0].toUpperCase() + t.slice(1)}
              </button>
            ))}
          </nav>
          {tab === "fields" && (
            <>
              <p className="muted">
                Choose the information this entity stores.
              </p>
              <div className="entity-toolbar entity-fields-toolbar">
                <label className="field">
                  <span>Name</span>
                  <input
                    value={draft.label}
                    onChange={(e) =>
                      setDraft({ ...draft, label: e.target.value })
                    }
                  />
                </label>
                <button
                  className="primary"
                  onClick={() =>
                    setEditField({
                      key: "",
                      label: "",
                      type: catalog.types[0],
                      max_length: 200,
                    })
                  }
                >
                  <Plus size={15} /> Add field
                </button>
              </div>
              <div className="entity-rows">
                {draft.fields.map((f: Item) => (
                  <EntityRow
                    key={f.key}
                    title={f.label}
                    summary={
                      <>
                        {f.type}
                        {f.required ? " · Required" : ""}
                        {f.default ? " · Default" : ""}
                        {f.calculation ? " · Calculated" : ""}
                      </>
                    }
                    edit={() => setEditField({ ...f })}
                    remove={() => remove("fields", f.key)}
                  />
                ))}
                {!draft.fields.length && (
                  <p className="entity-empty">
                    Add your first field to start defining this entity.
                  </p>
                )}
              </div>
              <details className="panel entity-settings">
                <summary>
                  <span>Entity settings</span>
                  <small>Ownership, default fields and sharing</small>
                </summary>
                <div className="entity-settings-content">
                  <div className="entity-form-grid">
                    <label className="field">
                      <span>Owner field</span>
                      <Select
                        value={draft.owner_field || ""}
                        onChange={(owner_field) =>
                          setDraft({ ...draft, owner_field })
                        }
                        options={[
                          { value: "", label: "None" },
                          ...draft.fields
                            .filter((f: Item) => f.type === "user")
                            .map((f: Item) => ({
                              value: f.key,
                              label: f.label,
                            })),
                        ]}
                      />
                    </label>
                    <label className="field">
                      <span>Owning team field</span>
                      <Select
                        value={draft.team_field || ""}
                        onChange={(team_field) =>
                          setDraft({ ...draft, team_field })
                        }
                        options={[
                          { value: "", label: "None" },
                          ...draft.fields
                            .filter((f: Item) => f.type === "team")
                            .map((f: Item) => ({
                              value: f.key,
                              label: f.label,
                            })),
                        ]}
                      />
                    </label>
                  </div>
                  <section className="entity-settings-group">
                    <h3>Default fields</h3>
                    <p className="muted">
                      Included when an App doesn't select specific fields.
                    </p>
                    <FieldChoices
                      fields={draft.fields}
                      value={draft.default_projection || []}
                      onChange={(v) =>
                        setDraft({ ...draft, default_projection: v })
                      }
                    />
                  </section>
                  <section className="entity-settings-group">
                    <label>
                      <input
                        type="checkbox"
                        checked={!!draft.sharing}
                        onChange={(e) =>
                          setDraft({ ...draft, sharing: e.target.checked })
                        }
                      />{" "}
                      Allow record sharing
                    </label>
                  </section>
                  <div className="entity-settings-footer">
                    <p className="muted">
                      API key <code>{draft.key}</code>
                    </p>
                    <button
                      className="secondary danger"
                      disabled={busy || !!record?.pending_version_id}
                      onClick={async () => {
                        const archived = record?.state === "archived";
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
                                state:
                                  record?.state === "archived"
                                    ? "active"
                                    : "archived",
                              },
                              saved.revision,
                            );
                            setRecord(v);
                          });
                      }}
                    >
                      {record?.state === "archived"
                        ? "Restore entity"
                        : "Archive entity"}
                    </button>
                  </div>
                </div>
              </details>
            </>
          )}
          {tab === "access" && (
            <>
              <div className="entity-toolbar">
                <p>
                  Access is denied until an allow rule matches. Administrative
                  roles do not automatically grant data access.
                </p>
                <button
                  className="primary"
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
                  <Plus size={15} /> Add access rule
                </button>
              </div>
              <div className="entity-rows">
                {draft.permissions.map((p: Item) => (
                  <EntityRow
                    key={p.key}
                    title={p.label}
                    summary={
                      <>
                        {p.relationship ||
                          p.roles
                            ?.map(
                              (id: string) =>
                                roles.find((r) => r.id === id)?.display_name ||
                                id,
                            )
                            .join(", ")}{" "}
                        · {p.operation} ·{" "}
                        {(p.fields || []).join(", ") || "Record"}
                      </>
                    }
                    edit={() => setEditPermission({ ...p })}
                    remove={() => remove("permissions", p.key)}
                  />
                ))}
                {!draft.permissions.length && (
                  <div className="entity-empty">
                    Add a Viewer or Editor rule by choosing who, what they can
                    do, and which fields.
                  </div>
                )}
              </div>
            </>
          )}
          {tab === "rules" && (
            <>
              <div className="entity-toolbar">
                <p>Rules apply to every write, from every App.</p>
                <button
                  className="primary"
                  onClick={() =>
                    setEditRule({
                      key: "",
                      name: "",
                      kind: "validation",
                      assert: {
                        op: "is_not_null",
                        args: [{ field: draft.fields[0]?.key || "" }],
                      },
                      message: "",
                    })
                  }
                >
                  <Plus size={15} /> Add rule
                </button>
              </div>
              <div className="entity-rows">
                {draft.fields
                  .filter((f: Item) => f.default || f.calculation)
                  .map((f: Item) => (
                    <EntityRow
                      key={f.key}
                      title={f.label}
                      summary={f.calculation ? "Calculation" : "Default"}
                      edit={() => setEditField({ ...f })}
                    />
                  ))}
                {draft.rules.map((rule: Item) => (
                  <EntityRow
                    key={rule.key}
                    title={rule.name || rule.key}
                    summary={
                      <>
                        {rule.kind === "change"
                          ? "Allowed change"
                          : "Validation"}{" "}
                        · {rule.message}
                      </>
                    }
                    edit={() => setEditRule({ ...rule })}
                    remove={() => remove("rules", rule.key)}
                  />
                ))}
                {!draft.rules.length &&
                  !draft.fields.some(
                    (f: Item) => f.default || f.calculation,
                  ) && (
                    <p className="entity-empty">
                      Add validation or allowed-change rules here. Set defaults
                      and calculations in a field's editor.
                    </p>
                  )}
              </div>
            </>
          )}
          {tab === "test" && (
            <EntityTest
              draft={draft}
              record={record!}
              tenant={tenant}
              space={space}
              me={me}
            />
          )}
          {review && (
            <EditorDialog
              title="Review and publish"
              close={() => setReview(false)}
            >
              <p>
                Apps inherit the published definition. Your current draft stays
                separate until publication succeeds.
              </p>
              <ReviewDiff before={record?.active_definition} after={draft} />
              {draft.fields.some(
                (f: Item) =>
                  f.calculation &&
                  draft.permissions.some(
                    (p: Item) =>
                      p.operation === "read" && p.fields?.includes(f.key),
                  ),
              ) && (
                <p className="muted">
                  Calculated fields with view access can reveal information
                  about their inputs. Review their access alongside the formula.
                </p>
              )}
              {record?.diagnostics?.length > 0 && (
                <p className="muted">
                  Saved draft has diagnostics. Validation below checks your
                  latest changes.
                </p>
              )}
              {record?.diagnostics?.map((d: Item, i: number) => (
                <p className="error" key={i}>
                  {d.path}: {d.message}
                </p>
              ))}
              <div className="entity-dialog-actions">
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() =>
                    void run(async () => {
                      const v = await api<Item>(
                        "/v1/entities/" + record!.id + "/validate",
                        "POST",
                        { definition: draft },
                      );
                      setRecord({ ...record, diagnostics: v.diagnostics });
                      if (!v.diagnostics.length)
                        setError("Validation passed. Ready to publish.");
                    })
                  }
                >
                  Validate
                </button>
                <button
                  className="primary"
                  disabled={busy || !!record?.pending_version_id}
                  onClick={() =>
                    void run(async () => {
                      const saved = dirty ? await save() : record!;
                      await api<Item>(
                        "/v1/entities/" + saved.id + "/publications",
                        "POST",
                        {},
                        saved.revision,
                      );
                      const v = await api<Item>("/v1/entities/" + saved.id);
                      setRecord(v);
                      setReview(false);
                    })
                  }
                >
                  Publish
                </button>
              </div>
            </EditorDialog>
          )}
          {record?.publications?.[0]?.state === "failed" && (
            <div className="error" role="alert">
              Publishing failed: {record.publications[0].error_message} Review
              changes and publish again.
            </div>
          )}
          {editField && (
            <EditorDialog
              title={editField.key ? "Edit field" : "Add field"}
              close={() => setEditField(null)}
            >
              <FieldEditor
                field={editField}
                fields={draft.fields}
                catalog={catalog}
                space={space}
                published={
                  !!record?.active_definition?.fields.some(
                    (f: Item) => f.key === editField.key,
                  )
                }
                save={(f) => {
                  replace("fields", editField.key, f);
                  setEditField(null);
                }}
              />
            </EditorDialog>
          )}
          {editPermission && (
            <EditorDialog
              title="Access rule"
              close={() => setEditPermission(null)}
            >
              <PermissionEditor
                value={editPermission}
                definition={draft}
                roles={roles}
                catalog={catalog}
                tenant={tenant}
                save={(v) => {
                  replace("permissions", editPermission.key, v);
                  setEditPermission(null);
                }}
              />
            </EditorDialog>
          )}
          {editRule && (
            <EditorDialog title="Business rule" close={() => setEditRule(null)}>
              <RuleEditor
                value={editRule}
                fields={draft.fields}
                catalog={catalog}
                save={(v) => {
                  replace("rules", editRule.key, v);
                  setEditRule(null);
                }}
              />
            </EditorDialog>
          )}
        </>
      )}
    </section>
  );
}
function EntityRow({
  title,
  summary,
  edit,
  remove,
}: {
  title: string;
  summary: React.ReactNode;
  edit: () => void;
  remove?: () => void;
}) {
  return (
    <article className="entity-row">
      <div className="entity-row-copy">
        <strong>{title}</strong>
        <small>{summary}</small>
      </div>
      <div className="entity-row-actions">
        <button
          className="secondary"
          aria-label={"Edit " + title}
          onClick={edit}
        >
          <Pencil size={13} /> Edit
        </button>
        {remove && (
          <button
            className="icon danger"
            aria-label={"Remove " + title}
            title={"Remove " + title}
            onClick={remove}
          >
            <Trash2 size={15} />
          </button>
        )}
      </div>
    </article>
  );
}
export function EditorDialog({
  title,
  close,
  children,
}: {
  title: string;
  close: () => void;
  children: React.ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const active = document.activeElement as HTMLElement;
    ref.current?.showModal();
    return () => {
      ref.current?.close();
      active?.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className="modal entity-dialog"
      aria-label={title}
      onCancel={(e) => {
        e.preventDefault();
        close();
      }}
    >
      <header>
        <h2>{title}</h2>
        <button className="icon" aria-label="Close editor" onClick={close}>
          <X size={18} />
        </button>
      </header>
      <div className="entity-dialog-body">{children}</div>
    </dialog>
  );
}
function FieldEditor({
  field,
  fields,
  catalog,
  space,
  published,
  save,
}: {
  field: Item;
  fields: Item[];
  catalog: EntityCatalog;
  space: string;
  published: boolean;
  save: (v: Item) => void;
}) {
  const [f, set] = useState<Item>({ ...field }),
    [error, setError] = useState(""),
    [choice, setChoice] = useState<string>((field.values || []).join(", "));
  const patch = (v: Item) => set({ ...f, ...v });
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (fields.some((x) => x.key === f.key && x.key !== field.key)) {
          setError(
            "Another field already uses this key. Choose a different key.",
          );
          return;
        }
        const value = { ...f };
        if (value.type === "choice")
          value.values = choice
            .split(",")
            .map((v) => v.trim())
            .filter(Boolean);
        if (value.type === "choice" && value.value_labels)
          value.value_labels = Object.fromEntries(
            Object.entries(value.value_labels).filter(([key]) =>
              value.values.includes(key),
            ),
          );
        save(value);
      }}
    >
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <label className="field">
        <span>Name</span>
        <input
          autoFocus
          required
          value={f.label}
          onChange={(e) =>
            patch({
              label: e.target.value,
              ...(!field.key ? { key: keyOf(e.target.value) } : {}),
            })
          }
        />
      </label>
      <label className="field">
        <span>Type</span>
        <Select
          value={f.type}
          disabled={published}
          onChange={(type) =>
            set({
              ...catalog.field_defaults[type],
              key: f.key,
              label: f.label,
            })
          }
          options={catalog.types.map((type: string) => ({
            value: type,
            label: fieldLabel(type),
          }))}
        />
      </label>
      <label>
        <input
          type="checkbox"
          checked={!!f.required}
          onChange={(e) => patch({ required: e.target.checked })}
        />
        Required
      </label>
      {f.type === "text" && (
        <label className="field">
          <span>Maximum length</span>
          <NumberInput
            required
            max={catalog.field_constraints.max_text_length}
            min={catalog.field_constraints.min_text_length}
            value={f.max_length}
            onChange={(n) => patch({ max_length: Number(n) })}
          />
        </label>
      )}
      {f.type === "decimal" && (
        <div className="entity-toolbar">
          {["precision", "scale"].map((key) => (
            <label className="field" key={key}>
              <span>{fieldLabel(key)}</span>
              <NumberInput
                required
                value={f[key]}
                onChange={(n) => patch({ [key]: Number(n) })}
              />
            </label>
          ))}
        </div>
      )}
      {f.type === "choice" && (
        <label className="field">
          <span>Choices, separated by commas</span>
          <input
            required
            value={choice}
            onChange={(e) => setChoice(e.target.value)}
          />
        </label>
      )}
      {f.type === "choice" && (
        <details>
          <summary>Choice display labels</summary>
          <p className="muted">Keep keys stable; change their labels here.</p>
          {choice
            .split(",")
            .map((v) => v.trim())
            .filter(Boolean)
            .map((key) => (
              <label className="field" key={key}>
                <span>{key}</span>
                <input
                  value={choiceLabel(f, key)}
                  onChange={(e) =>
                    patch({
                      value_labels: {
                        ...f.value_labels,
                        [key]: e.target.value,
                      },
                    })
                  }
                />
              </label>
            ))}
        </details>
      )}
      {f.type === "reference" && (
        <label className="field">
          <span>Related entity</span>
          <RemoteSelect
            path={"/v1/spaces/" + space + "/entities"}
            label="Related entity"
            value={f.target || ""}
            initial={[]}
            labelKey="label"
            onChange={(value) => patch({ target: value })}
          />
        </label>
      )}
      {["integer", "decimal"].includes(f.type) && (
        <div className="entity-toolbar">
          {["minimum", "maximum"].map((key) => (
            <label className="field" key={key}>
              <span>{fieldLabel(key)}</span>
              <input
                value={f[key] ?? ""}
                onChange={(e) =>
                  patch({
                    [key]:
                      e.target.value === ""
                        ? undefined
                        : f.type === "integer"
                          ? Number(e.target.value)
                          : e.target.value,
                  })
                }
              />
            </label>
          ))}
        </div>
      )}
      <details>
        <summary>Defaults, calculation and query options</summary>
        <label className="field">
          <span>Value source</span>
          <Select
            value={
              f.calculation ? "calculation" : f.default ? "default" : "input"
            }
            onChange={(source) =>
              patch({
                default: source === "default" ? { literal: "" } : undefined,
                calculation:
                  source === "calculation"
                    ? { field: fields[0]?.key || "" }
                    : undefined,
              })
            }
            options={[
              { value: "input", label: "User input" },
              { value: "default", label: "Default when omitted" },
              { value: "calculation", label: "Calculated" },
            ]}
          />
        </label>
        {(f.default || f.calculation) && (
          <Expression
            value={f.calculation || f.default}
            onChange={(value) =>
              patch(f.calculation ? { calculation: value } : { default: value })
            }
            fields={fields}
            catalog={catalog}
            label={f.calculation ? "Formula" : "Default"}
            formula={!!f.calculation}
            defaultOnly={!!f.default}
            contextType={f.default ? f.type : undefined}
            allowContext={!f.calculation}
          />
        )}
        {["filterable", "sortable"].map((key) => (
          <label key={key}>
            <input
              type="checkbox"
              checked={!!f[key]}
              onChange={(e) => patch({ [key]: e.target.checked })}
            />
            {key === "filterable" ? "Allow filtering" : "Allow sorting"}
          </label>
        ))}
        <label className="field">
          <span>Uniqueness</span>
          <Select
            value={f.unique || ""}
            onChange={(unique) => patch({ unique })}
            options={[
              { value: "", label: "None" },
              { value: "entity", label: "Across this entity" },
              { value: "owner", label: "Per owner" },
              { value: "team", label: "Per team" },
            ]}
          />
        </label>
        <label className="field">
          <span>Stable key</span>
          <input
            required
            pattern="[a-z][a-z0-9_]*"
            value={f.key}
            disabled={!!field.key}
            onChange={(e) => patch({ key: e.target.value })}
          />
        </label>
      </details>
      <button className="primary">Save field</button>
    </form>
  );
}
function PermissionEditor({
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
          value={p.label}
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
          value={p.roles ? "roles" : p.relationship}
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
      {p.roles && (
        <div className="field-choices">
          {roles.map((role) => (
            <label key={role.id}>
              <input
                type="checkbox"
                checked={p.roles.includes(role.id)}
                onChange={(e) =>
                  patch({
                    roles: e.target.checked
                      ? [...p.roles, role.id]
                      : p.roles.filter((v: string) => v !== role.id),
                  })
                }
              />
              {role.display_name}
            </label>
          ))}
          {!roles.length && (
            <p>Create a business role in Space → Access → Manage roles.</p>
          )}
        </div>
      )}
      {["user", "team"].includes(p.relationship) && (
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
            value={p.subject || ""}
            onChange={(v) => patch({ subject: v })}
          />
        </label>
      )}
      <label className="field">
        <span>Can</span>
        <Select
          value={p.operation}
          onChange={(operation) => patch({ operation })}
          options={catalog.operations.map((op: string) => ({
            value: op,
            label: op === "acl.manage" ? "Manage record access (advanced)" : op,
          }))}
        />
      </label>
      {["read", "create", "update"].includes(p.operation) && (
        <>
          <p>Which fields</p>
          <FieldChoices
            fields={definition.fields.filter(
              (f: Item) => p.operation === "read" || !f.calculation,
            )}
            value={p.fields || []}
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
                    args: [
                      { field: definition.fields[0]?.key || "" },
                      { literal: "" },
                    ],
                  }
                : undefined,
            })
          }
        />{" "}
        Limit to records matching a condition
      </label>
      {p.when && (
        <Expression
          value={p.when}
          onChange={(v) => patch({ when: v })}
          fields={definition.fields}
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
                          { field: "old." + (definition.fields[0]?.key || "") },
                          { literal: "" },
                        ],
                      }
                    : undefined,
                })
              }
            />{" "}
            Authorize a particular change
          </label>
          {p.change_when && (
            <Expression
              value={p.change_when}
              onChange={(v) => patch({ change_when: v })}
              fields={definition.fields}
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
            fields={definition.permissions
              .filter(
                (v: Item) =>
                  v.roles?.length &&
                  !["create", "acl.manage"].includes(v.operation),
              )
              .map((v: Item) => ({ key: v.key, label: v.label }))}
            value={p.delegable || []}
            onChange={(v) => patch({ delegable: v })}
          />
        </>
      )}
      <button className="primary">Save access rule</button>
    </form>
  );
}
function RuleEditor({
  value,
  fields,
  catalog,
  save,
}: {
  value: Item;
  fields: Item[];
  catalog: EntityCatalog;
  save: (v: Item) => void;
}) {
  const [r, set] = useState({ ...value });
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        save(r);
      }}
    >
      <label className="field">
        <span>Name</span>
        <input
          autoFocus
          required
          value={r.name || ""}
          onChange={(e) =>
            set({
              ...r,
              name: e.target.value,
              ...(!value.key ? { key: keyOf(e.target.value) } : {}),
            })
          }
        />
      </label>
      <label className="field">
        <span>Rule type</span>
        <Select
          value={r.kind}
          onChange={(kind) => set({ ...r, kind })}
          options={[
            { value: "validation", label: "Validate resulting record" },
            { value: "change", label: "Allowed change" },
          ]}
        />
      </label>
      <label>
        <input
          type="checkbox"
          checked={!!r.when}
          onChange={(e) =>
            set({
              ...r,
              when: e.target.checked
                ? {
                    op: "eq",
                    args: [{ field: fields[0]?.key || "" }, { literal: "" }],
                  }
                : undefined,
            })
          }
        />{" "}
        Apply only when
      </label>
      {r.when && (
        <Expression
          value={r.when}
          onChange={(v) => set({ ...r, when: v })}
          fields={fields}
          catalog={catalog}
          transition={r.kind === "change"}
          label="When"
          allowContext={false}
        />
      )}
      <Expression
        value={r.assert}
        onChange={(v) => set({ ...r, assert: v })}
        fields={fields}
        catalog={catalog}
        transition={r.kind === "change"}
        label="Must be true"
        allowContext={false}
      />
      <label className="field">
        <span>Message shown to the user</span>
        <input
          required
          value={r.message}
          onChange={(e) => set({ ...r, message: e.target.value })}
        />
      </label>
      <label className="field">
        <span>Show error on field</span>
        <Select
          value={r.field || ""}
          onChange={(field) => set({ ...r, field })}
          options={[
            { value: "", label: "Whole record" },
            ...fields.map((f) => ({ value: f.key, label: f.label })),
          ]}
        />
      </label>
      <button className="primary">Save rule</button>
    </form>
  );
}
function EntityTest({
  draft,
  record,
  tenant,
  space,
  me,
}: {
  draft: Item;
  record: Item;
  tenant: string;
  space: string;
  me: Item;
}) {
  const [scenario, set] = useState<Item>({
      user_id: me.id,
      app_id: "",
      operation: "create",
      record: {},
      values: {},
    }),
    [result, setResult] = useState<Item | null>(null),
    [error, setError] = useState("");
  return (
    <form
      className="panel entity-test"
      onSubmit={(e) => {
        e.preventDefault();
        setError("");
        api<Item>("/v1/entities/" + record.id + "/test", "POST", {
          definition: draft,
          ...scenario,
        })
          .then(setResult)
          .catch((e) => setError(e.message));
      }}
    >
      <h3>Try a synthetic scenario</h3>
      <p>
        This uses the backend evaluator with sample values. It never reads or
        writes business records.
      </p>
      <label className="field">
        <span>Test as user</span>
        <RemoteSelect
          initial={[]}
          path={"/v1/tenants/" + tenant + "/recipients"}
          label="Test as user"
          value={scenario.user_id}
          onChange={(v) => set({ ...scenario, user_id: v })}
        />
      </label>
      <label className="field">
        <span>App context (optional)</span>
        <RemoteSelect
          initial={[]}
          path={"/v1/tenants/" + tenant + "/apps?space_id=" + space}
          label="App context"
          value={scenario.app_id}
          onChange={(v) => set({ ...scenario, app_id: v })}
        />
      </label>
      <label className="field">
        <span>Operation</span>
        <Select
          value={scenario.operation}
          onChange={(operation) => set({ ...scenario, operation })}
          options={["create", "read", "update", "delete"].map((v) => ({
            value: v,
            label: v,
          }))}
        />
      </label>
      {scenario.operation !== "create" && (
        <>
          <h4>Existing sample record</h4>
          <ValueInputs
            fields={draft.fields}
            value={scenario.record}
            onChange={(v) => set({ ...scenario, record: v })}
          />
        </>
      )}
      {["create", "update"].includes(scenario.operation) && (
        <>
          <h4>Proposed values</h4>
          <ValueInputs
            fields={draft.fields}
            value={scenario.values}
            onChange={(v) => set({ ...scenario, values: v })}
          />
        </>
      )}
      <button className="primary">Run test</button>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {result && (
        <section className="entity-result">
          <h3>{result.allowed ? "Allowed" : "Denied or invalid"}</h3>
          {result.error && <p>{result.error}</p>}
          {result.diagnostics?.map((d: Item, i: number) => (
            <p key={i}>
              {d.path}: {d.message}
            </p>
          ))}
          <p>Not checked: {result.not_checked?.join(", ")}</p>
          {result.permission_paths?.map((p: Item) => (
            <p key={p.key}>
              {p.matches_complete_path ? "✓" : "–"} {p.label} ·{" "}
              {(p.fields || []).join(", ") || "Record"}
            </p>
          ))}
          {result.candidate && (
            <pre>{JSON.stringify(result.candidate, null, 2)}</pre>
          )}
        </section>
      )}
    </form>
  );
}
function ReviewDiff({ before, after }: { before?: Item; after: Item }) {
  if (!before)
    return (
      <div className="panel">
        <strong>New entity</strong>
        <p>
          {after.fields.length} fields · {after.permissions.length} access rules
          · {after.rules.length} business rules
        </p>
        <p>No data is accessible until an access rule grants it.</p>
      </div>
    );
  return (
    <ul className="entity-diff">
      {["fields", "permissions", "rules"].flatMap((k) => {
        const previous = before[k] || [],
          next = after[k] || [];
        return [
          ...next
            .filter((n: Item) => !previous.some((p: Item) => p.key === n.key))
            .map((v: Item) => (
              <li key={k + v.key}>
                Add {k}: {v.label || v.name || v.key}
                {k === "fields" ? " — select access explicitly" : ""}
              </li>
            )),
          ...next
            .filter((n: Item) =>
              previous.some(
                (p: Item) =>
                  p.key === n.key && JSON.stringify(n) !== JSON.stringify(p),
              ),
            )
            .map((v: Item) => (
              <li key={k + v.key}>
                Change {k}: {v.label || v.name || v.key}
                {k === "permissions"
                  ? " — review recipients, fields and conditions; existing shares may become inactive"
                  : ""}
              </li>
            )),
          ...previous
            .filter((p: Item) => !next.some((n: Item) => n.key === p.key))
            .map((v: Item) => (
              <li key={k + v.key}>
                Remove {k}: {v.label || v.name || v.key}
                {k === "fields" ? " — unsupported for published fields" : ""}
              </li>
            )),
        ];
      })}
      <li>
        Every inheriting App uses the newly published version. Existing records
        are checked before activation.
      </li>
    </ul>
  );
}
