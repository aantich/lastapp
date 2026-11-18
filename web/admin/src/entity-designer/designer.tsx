import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { apiKey, queryClient, useApiQuery } from "../query";
import {
  items as itemsOf,
  num,
  text,
  type EntityCatalog,
  type Item,
  type Page,
} from "../api-types";
import { useAction } from "../use-action";
import { confirmAction, Alert, Tabs } from "../ui";
import { beforeNavigateEvent, type BeforeNavigateEvent } from "../console-url";
import { api } from "../api";
import { definitionOf, discardChanges, type Designer } from "./shared";
import { EditorDialog } from "./editor-dialog";
import { EntityList } from "./entity-list";
import { AccessTab, FieldsTab, RulesTab } from "./tabs";
import { ReviewDialog } from "./review";
import { EntityTest } from "./entity-test";
import { FieldEditor } from "./field-editor";
import { PermissionEditor } from "./permission-editor";
import { RuleEditor } from "./rule-editor";

// The Space entity designer: the entity list, and for an open entity its
// draft, tabs, editors and publication. Panels live in sibling files and read
// the open entity through a Designer object built here.
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
    [review, setReview] = useState(false);
  const [editField, setEditField] = useState<Item | null>(null),
    [editPermission, setEditPermission] = useState<Item | null>(null),
    [editRule, setEditRule] = useState<Item | null>(null);
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
      setDraft(definitionOf(linkedQuery.data.draft_json));
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
    if (!dirty) setDraft(definitionOf(v.draft_json));
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
      setDraft(definitionOf(v.draft_json));
      history.replaceState(
        null,
        "",
        location.pathname + "?entity=" + encodeURIComponent(text(v.id)),
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
      num(record!.revision),
    );
    setRecord(v);
    setDraft(definitionOf(v.draft_json));
    return v;
  }
  const replace = (list: string, key: string, v: Item) =>
    setDraft((d) => ({
      ...d,
      ...(list === "fields" && itemsOf(d!.fields).length === 0
        ? { default_projection: [v.key] }
        : {}),
      [list]: itemsOf(d![list]).some((x) => x.key === key)
        ? itemsOf(d![list]).map((x) => (x.key === key ? v : x))
        : [...itemsOf(d![list]), v],
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
        [list]: itemsOf(d![list]).filter((x) => x.key !== key),
      }));
  };
  const d: Designer | null =
    catalog && draft
      ? {
          space,
          tenant,
          me,
          catalog,
          roles,
          record,
          setRecord,
          draft,
          setDraft,
          dirty,
          busy,
          run,
          save,
          remove,
          setError,
          setReview,
          setEditField,
          setEditPermission,
          setEditRule,
        }
      : null;
  const shownError = error || readError?.message || "";
  if (!catalog)
    return (
      <p role={shownError ? "alert" : undefined}>
        {shownError || "Loading entity designer…"}
      </p>
    );
  return (
    <section className="entity-designer">
      {shownError && <Alert>{shownError}</Alert>}
      {!draft || !d ? (
        <EntityList
          space={space}
          catalog={catalog}
          items={items}
          busy={busy}
          run={run}
          open={(id) => void open(id)}
          opened={(v) => {
            setRecord(v);
            setDraft(definitionOf(v.draft_json));
          }}
        />
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
                <h2>{text(draft.label)}</h2>
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
          <Tabs
            label="Entity designer"
            className="entity-tabs"
            items={["fields", "access", "rules", "test"].map((id) => ({
              id,
              label: id[0].toUpperCase() + id.slice(1),
            }))}
            value={tab}
            onChange={setTab}
          />
          {tab === "fields" && <FieldsTab d={d} />}
          {tab === "access" && <AccessTab d={d} />}
          {tab === "rules" && <RulesTab d={d} />}
          {tab === "test" && (
            <EntityTest
              draft={draft}
              record={record!}
              tenant={tenant}
              space={space}
              me={me}
            />
          )}
          {review && <ReviewDialog d={d} />}
          {itemsOf(record?.publications)[0]?.state === "failed" && (
            <Alert>
              Publishing failed:{" "}
              {text(itemsOf(record?.publications)[0]?.error_message)} Review
              changes and publish again.
            </Alert>
          )}
          {editField && (
            <EditorDialog
              title={editField.key ? "Edit field" : "Add field"}
              close={() => setEditField(null)}
            >
              <FieldEditor
                field={editField}
                fields={itemsOf(draft.fields)}
                catalog={catalog}
                space={space}
                published={itemsOf(
                  definitionOf(record?.active_definition)?.fields,
                ).some((f) => f.key === editField.key)}
                save={(f) => {
                  replace("fields", text(editField.key), f);
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
                  replace("permissions", text(editPermission.key), v);
                  setEditPermission(null);
                }}
              />
            </EditorDialog>
          )}
          {editRule && (
            <EditorDialog title="Business rule" close={() => setEditRule(null)}>
              <RuleEditor
                value={editRule}
                fields={itemsOf(draft.fields)}
                catalog={catalog}
                save={(v) => {
                  replace("rules", text(editRule.key), v);
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
