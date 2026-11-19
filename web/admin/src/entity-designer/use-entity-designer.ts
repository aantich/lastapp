import { useEffect, useRef, useState } from "react";
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
import { confirmAction } from "../ui";
import { api } from "../api";
import { definitionOf, discardChanges, type Designer } from "./shared";
import { publishableChanges } from "./draft-changes";
import { useEntityLink } from "./use-entity-link";
import { usePublicationPoll } from "./use-publication-poll";
import { useUnsavedGuard } from "./use-unsaved-guard";

// The entity designer's state: the open record and its draft, the reads it
// needs, the editor dialogs and the actions on them. EntityDesigner renders
// it; panels read it through the Designer object built here.
export function useEntityDesigner(space: string, tenant: string, me: Item) {
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
  const { linkedQuery, linkHandled } = useEntityLink(
    space,
    record,
    setRecord,
    setDraft,
    setTab,
  );
  const pendingQuery = usePublicationPoll(
    record,
    dirty,
    setRecord,
    setDraft,
    load,
  );
  const readError =
    catalogQuery.error ||
    rolesQuery.error ||
    listQuery.error ||
    linkedQuery.error ||
    pendingQuery.error;
  useUnsavedGuard(dirty);
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
  const changes =
    record && draft ? publishableChanges(draft, record, dirty) : null;
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
          changes,
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
  return {
    d,
    record,
    draft,
    dirty,
    tab,
    setTab,
    review,
    creating,
    setCreating,
    editField,
    editPermission,
    editRule,
    catalog,
    items,
    listLoaded: !!listQuery.data,
    spaceName: text(spaceQuery.data?.display_name),
    shownError: error || readError?.message || "",
    busy,
    run,
    load,
    show,
    open,
    replace,
  };
}
