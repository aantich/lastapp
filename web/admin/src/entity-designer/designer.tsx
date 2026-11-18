import { useEffect, useRef, useState } from "react";
import { Database, Plus } from "lucide-react";
import { apiKey, queryClient, useApiQuery } from "../query";
import {
  isItem,
  items as itemsOf,
  num,
  text,
  type EntityCatalog,
  type Item,
  type Page,
} from "../api-types";
import { useAction } from "../use-action";
import {
  Alert,
  Badge,
  Button,
  Card,
  DesignerBody,
  DesignerPage,
  EmptyState,
  NavList,
  NavListNote,
  PageHeader,
  Panel,
  RecordHeader,
  StatusBadge,
  Stepper,
  Tabs,
  confirmAction,
} from "../ui";
import { beforeNavigateEvent, type BeforeNavigateEvent } from "../console-url";
import { api } from "../api";
import {
  definitionOf,
  discardChanges,
  draftChanges,
  ruleRows,
  type Designer,
} from "./shared";
import { EditorDialog } from "./editor-dialog";
import { NewEntityDialog } from "./entity-list";
import {
  AccessTab,
  EntitySettings,
  FieldsTab,
  HistoryTab,
  RulesTab,
} from "./tabs";
import { ReviewDialog } from "./review";
import { publishSteps } from "./pipeline";
import { EntityTest } from "./entity-test";
import { FieldEditor } from "./field-editor";
import { PermissionEditor } from "./permission-editor";
import { RuleEditor } from "./rule-editor";

const plural = (n: number, word: string) =>
  n + " " + word + (n === 1 ? "" : "s");

// The Space entity designer: the entity list beside the open entity's
// header, tabs, editors and publication. Panels live in sibling files and
// read the open entity through a Designer object built here.
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
    [review, setReview] = useState(false),
    [creating, setCreating] = useState(false);
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
    spaceQuery = useApiQuery<Item>(
      "/v1/tenants/" + tenant + "/spaces/" + space,
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
    [linkHandled, setLinkHandled] = useState(!linked);
  if (!linkHandled && (linkedQuery.data || linkedQuery.error)) {
    setLinkHandled(true);
    if (!record && linkedQuery.data?.space_id === space) {
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
  const show = (v: Item) => {
    setRecord(v);
    setDraft(definitionOf(v.draft_json));
    history.replaceState(
      null,
      "",
      location.pathname + "?entity=" + encodeURIComponent(text(v.id)),
    );
    setReview(false);
    setTab("fields");
  };
  async function open(id: string) {
    if (dirty && !(await discardChanges())) return;
    await run(async () => {
      show(await api<Item>("/v1/entities/" + id));
    });
  }
  // Without a link, the first entity opens, as the list's selection.
  const autoOpened = useRef(false);
  useEffect(() => {
    if (autoOpened.current || !linkHandled || record || !items.length) return;
    autoOpened.current = true;
    void open(text(items[0].id));
  });
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
          entities: items,
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
  if (!catalog || !listQuery.data)
    return shownError ? (
      <Alert>{shownError}</Alert>
    ) : (
      <Card>
        <EmptyState compact>Loading entity designer…</EmptyState>
      </Card>
    );
  const spaceName = text(spaceQuery.data?.display_name);
  const nav = (
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
          <Button onClick={() => setCreating(true)} disabled={busy}>
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
  const creator = creating && (
    <NewEntityDialog
      space={space}
      catalog={catalog}
      run={run}
      close={() => setCreating(false)}
      opened={(v) => {
        show(v);
        void load();
      }}
    />
  );
  if (!draft || !d || !record)
    return (
      <DesignerPage>
        {shownError && <Alert>{shownError}</Alert>}
        <PageHeader title={spaceName ? spaceName + " entities" : "Entities"} />
        <DesignerBody nav={nav}>
          {items.length ? (
            <Card>
              <EmptyState compact>Opening entity…</EmptyState>
            </Card>
          ) : (
            <Card>
              <EmptyState
                icon={<Database size={28} aria-hidden />}
                title="No entities yet"
                actions={
                  <Button
                    variant="primary"
                    icon={<Plus size={15} aria-hidden />}
                    onClick={() => setCreating(true)}
                  >
                    New entity
                  </Button>
                }
              >
                Describe your data once. Every App in this Space shares its API,
                access rules and validation.
              </EmptyState>
            </Card>
          )}
        </DesignerBody>
        {creator}
      </DesignerPage>
    );
  const publications = itemsOf(record.publications),
    latest = publications[0],
    active = publications.find((p) => p.id === record.active_version_id),
    changes = draftChanges(draft, record.active_definition),
    pending = !!record.pending_version_id,
    archived = record.state === "archived";
  const label = text(draft.label) || text(record.label);
  const publishedOn = active
    ? new Date(text(active.created_at)).toLocaleDateString(undefined, {
        day: "numeric",
        month: "short",
        year: "numeric",
      })
    : "";
  const meta = [
    text(draft.key),
    record.active_version_id
      ? "published" + (publishedOn ? " " + publishedOn : "")
      : "not published",
  ].join(" · ");
  // The latest publication still describes this draft when its hash is the
  // saved draft's: then its state (pending or failed) is the pipeline's.
  const draftHash = isItem(record.draft_contract)
      ? text(record.draft_contract.definition_hash)
      : "",
    current = !!latest && !dirty && text(latest.definition_hash) === draftHash,
    failed = latest?.state === "failed" && (pending || current);
  const showPipeline = pending || dirty || changes !== 0 || failed;
  const diagnostics = Array.isArray(record.diagnostics)
    ? record.diagnostics
    : [];
  const steps = publishSteps(
    pending || failed
      ? { dirty: false, publication: latest }
      : {
          dirty,
          validation: dirty ? "" : diagnostics.length ? "failed" : "passed",
          changes,
        },
  );
  const rulesCount = ruleRows(d).length;
  return (
    <DesignerPage>
      {shownError && <Alert>{shownError}</Alert>}
      <RecordHeader
        initial={label.slice(0, 1).toUpperCase()}
        title={label}
        meta={meta}
        chips={
          pending ? (
            <StatusBadge state="publishing" />
          ) : archived ? (
            <StatusBadge state="archived" />
          ) : changes === null ? (
            <Badge pill tone="info">
              Draft
            </Badge>
          ) : changes > 0 ? (
            <Badge pill tone="info">
              Draft · {plural(changes, "change")}
            </Badge>
          ) : undefined
        }
        actions={
          <>
            <Button onClick={() => setTab("test")}>Test as user…</Button>
            {dirty && (
              <Button
                disabled={busy}
                onClick={() =>
                  void run(async () => {
                    await save();
                  })
                }
              >
                Save draft
              </Button>
            )}
            <Button
              variant="primary"
              disabled={busy || pending || archived}
              onClick={() => setReview(true)}
            >
              Review &amp; publish
            </Button>
          </>
        }
      />
      <Tabs
        label="Entity sections"
        variant="line"
        items={[
          { id: "fields", label: "Fields" },
          { id: "access", label: "Access" },
          {
            id: "rules",
            label: "Rules",
            count: rulesCount || undefined,
          },
          { id: "test", label: "Test" },
          { id: "history", label: "History" },
        ]}
        value={tab}
        onChange={setTab}
      >
        <DesignerBody nav={nav}>
          {tab === "fields" && (
            <>
              <FieldsTab d={d} />
              {showPipeline && (
                <Panel
                  title={
                    (pending ? "Publishing" : "Publish changes") +
                    " · live data keeps working while storage is prepared"
                  }
                >
                  <Stepper label="Publication" steps={steps} />
                  {failed && (
                    <Alert live={false}>
                      Publishing failed: {text(latest.error_message)} Review
                      changes and publish again.
                    </Alert>
                  )}
                </Panel>
              )}
              <EntitySettings d={d} />
            </>
          )}
          {tab === "access" && <AccessTab d={d} />}
          {tab === "rules" && <RulesTab d={d} />}
          {tab === "test" && (
            <EntityTest
              draft={draft}
              record={record}
              tenant={tenant}
              space={space}
              me={me}
            />
          )}
          {tab === "history" && <HistoryTab d={d} />}
        </DesignerBody>
      </Tabs>
      {creator}
      {review && <ReviewDialog d={d} />}
      {editField && (
        <EditorDialog
          title={editField.key ? "Edit field" : "Add field"}
          close={() => setEditField(null)}
        >
          <FieldEditor
            field={editField}
            fields={itemsOf(d.draft.fields)}
            catalog={catalog}
            space={space}
            published={itemsOf(
              definitionOf(record.active_definition)?.fields,
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
          description="Who can do what, on which fields and which records."
          close={() => setEditPermission(null)}
        >
          <PermissionEditor
            value={editPermission}
            definition={d.draft}
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
            fields={itemsOf(d.draft.fields)}
            catalog={catalog}
            save={(v) => {
              replace("rules", text(editRule.key), v);
              setEditRule(null);
            }}
          />
        </EditorDialog>
      )}
    </DesignerPage>
  );
}
