import type { Item } from "../api-types";
import {
  Alert,
  Card,
  Columns,
  DesignerBody,
  DesignerPage,
  EmptyState,
  PageHeader,
  Tabs,
} from "../ui";
import { NewEntityDialog } from "./entity-list";
import {
  AccessTab,
  EntitySettings,
  FieldsTab,
  HistoryTab,
  PathsSummary,
  RulesTab,
} from "./tabs";
import { ReviewDialog } from "./review";
import { EntityTest } from "./entity-test";
import { ruleRows } from "./rule-rows";
import { entityTabs } from "./entity-tabs";
import { entityStatus } from "./entity-status";
import { useEntityDesigner } from "./use-entity-designer";
import { EntityHeader } from "./entity-header";
import { PublishPanel } from "./publish-panel";
import { EntityEditors } from "./entity-editors";
import { EntityNav } from "./entity-nav";
import { NoEntities } from "./no-entities";

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
  const {
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
    listLoaded,
    spaceName,
    shownError,
    busy,
    run,
    load,
    show,
    open,
    replace,
  } = useEntityDesigner(space, tenant, me);
  if (!catalog || !listLoaded)
    return shownError ? (
      <Alert>{shownError}</Alert>
    ) : (
      <Card>
        <EmptyState compact>Loading entity designer…</EmptyState>
      </Card>
    );
  const create = () => setCreating(true);
  const nav = (
    <EntityNav
      spaceName={spaceName}
      items={items}
      record={record}
      draft={draft}
      dirty={dirty}
      catalog={catalog}
      busy={busy}
      open={open}
      create={create}
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
            <NoEntities create={create} />
          )}
        </DesignerBody>
        {creator}
      </DesignerPage>
    );
  const status = entityStatus(record, dirty, d.changes);
  const rulesCount = ruleRows(d).length;
  return (
    <DesignerPage>
      {shownError && <Alert>{shownError}</Alert>}
      <EntityHeader d={d} status={status} setTab={setTab} />
      <Tabs
        label="Entity sections"
        variant="line"
        items={entityTabs.map(([id, label]) => ({
          id,
          label,
          count: id === "rules" ? rulesCount || undefined : undefined,
        }))}
        value={tab}
        onChange={setTab}
      >
        <DesignerBody nav={nav}>
          {tab === "fields" && (
            <>
              <Columns
                gap="md"
                tracks={[
                  { grow: 3, min: "3xl" },
                  { grow: 2, min: "lg" },
                ]}
              >
                <FieldsTab d={d} />
                <PathsSummary d={d} />
              </Columns>
              {status.showPipeline && (
                <PublishPanel record={record} status={status} />
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
      <EntityEditors
        d={d}
        editField={editField}
        editPermission={editPermission}
        editRule={editRule}
        replace={replace}
      />
    </DesignerPage>
  );
}
