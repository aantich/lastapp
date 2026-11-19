import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "../button";
import {
  Callout,
  ChipRow,
  DesignerBody,
  DesignerNote,
  DesignerPage,
  Panel,
  PathCard,
  RecordHeader,
  ResultTile,
  ResultTiles,
} from "../designer-parts";
import { GridTable } from "../grid-table";
import { NavList, NavListNote } from "../nav-list";
import { Stepper } from "../stepper";
import { Badge } from "../status";
import { Tabs } from "../tabs";

// The entity designer (entity-designer/designer.tsx) from kit parts: the
// record header, the section tabs, the record list beside the open record,
// the definition grid, the publish pipeline, access paths and results. The
// content is neutral sample data; the real screen derives every value.
const meta = { title: "Patterns/Entity designer" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

const nav = (
  <NavList
    label="Sales entities"
    items={[
      { id: "a", label: "Account" },
      { id: "o", label: "Order", dot: true, dotLabel: ", unpublished changes" },
      { id: "p", label: "Product" },
    ]}
    value="o"
    onPick={() => {}}
    footer={
      <>
        <Button>+ New entity</Button>
        <NavListNote>Templates: Account, Order</NavListNote>
      </>
    }
  />
);

export const Fields: Story = {
  render: () => (
    <DesignerPage>
      <RecordHeader
        initial="O"
        title="Order"
        meta="order · published v3"
        chips={
          <Badge pill tone="info">
            Draft · 2 changes
          </Badge>
        }
        actions={
          <>
            <Button>Test as user…</Button>
            <Button variant="primary">Review &amp; publish</Button>
          </>
        }
      />
      <Tabs
        label="Entity sections"
        variant="line"
        items={[
          { id: "fields", label: "Fields" },
          { id: "access", label: "Access" },
          { id: "rules", label: "Rules", count: 2 },
        ]}
        value="fields"
        onChange={() => {}}
      >
        <DesignerBody nav={nav}>
          <Panel
            flush
            title="Fields"
            actions={<Button size="sm">Add field</Button>}
          >
            <GridTable
              label="Fields"
              columns={[
                {
                  key: "field",
                  header: "Field",
                  track: "minmax(var(--ui-measure-2xs),1.4fr)",
                },
                {
                  key: "type",
                  header: "Type",
                  track: "minmax(var(--ui-col-label),1fr)",
                },
                {
                  key: "required",
                  header: "Required",
                  track: "var(--ui-col-flag)",
                },
                {
                  key: "note",
                  header: "Behaviour",
                  track: "minmax(var(--ui-measure-xs),2fr)",
                  muted: true,
                },
              ]}
              rows={[
                {
                  key: "title",
                  cells: {
                    field: <code>title</code>,
                    type: "text",
                    required: "Yes",
                    note: "Up to 120 characters",
                  },
                },
                {
                  key: "total",
                  highlight: true,
                  cells: {
                    field: (
                      <>
                        <code>total</code>{" "}
                        <Badge pill tone="info">
                          new
                        </Badge>
                      </>
                    ),
                    type: "decimal · calculated",
                    required: "—",
                    note: <code>Quantity × Price</code>,
                  },
                },
              ]}
            />
          </Panel>
          <Panel
            title="Publish v4"
            lead="Live data keeps working while storage is prepared."
          >
            <Stepper
              label="Publication"
              steps={[
                { id: "d", label: "Drafted", state: "done", note: "2 changes" },
                {
                  id: "v",
                  label: "Validated",
                  state: "done",
                  note: "No problems found",
                },
                {
                  id: "r",
                  label: "Review",
                  state: "active",
                  note: "Waiting for you",
                },
                {
                  id: "p",
                  label: "Prepare storage",
                  state: "pending",
                  note: "Live data unaffected",
                },
                {
                  id: "u",
                  label: "Published",
                  state: "pending",
                  note: "Apps switch over",
                },
              ]}
            />
          </Panel>
        </DesignerBody>
      </Tabs>
    </DesignerPage>
  ),
};

export const Access: Story = {
  render: () => (
    <Panel
      title="Permission paths"
      lead="Fields and their record predicates stay together."
    >
      <PathCard
        who="Reviewer"
        operations={["read"]}
        conditions={['where Status ≠ "closed"']}
        fields="Fields: title, status"
      />
      <PathCard
        who="Record owner"
        whoTone="success"
        operations={["update"]}
        fields="Fields: title"
      />
      <Callout
        title="total is not in any read path yet"
        action={<Button size="sm">Add to Reviewer</Button>}
      >
        New fields never join existing allowlists automatically.
      </Callout>
    </Panel>
  ),
};

export const RulesAndResult: Story = {
  render: () => (
    <Panel title="Rules">
      <ChipRow chip="validation" expression="Quantity ≥ 1" note="on save" />
      <ChipRow
        chip="calculation"
        expression="Total = Quantity × Price"
        note="stored"
      />
      <ResultTiles>
        <ResultTile label="Result">Allowed</ResultTile>
        <ResultTile label="Fields" size="mono" grow={2}>
          title, status
        </ResultTile>
      </ResultTiles>
      <DesignerNote>Allowed through Review orders.</DesignerNote>
    </Panel>
  ),
};
