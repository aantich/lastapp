import { Button, CellActions, CellMono, RowTable, StatusBadge } from "../ui";
import { useApiQuery } from "../query";
import { text, type Page } from "../api-types";
import { states, type TenantScreen } from "./screen";

// The App's micro-apps in navigation order. The artifact reference is
// Tenant-authored metadata, shown as text and never fetched.
export function MicroApps({ t }: { t: TenantScreen }) {
  const { app, base, edit, scoped } = t;
  const list = useApiQuery<Page>(
    `${base}/microapps?app_id=${text(app.id)}&limit=200`,
  );
  const manage = scoped("microapp.manage");
  return (
    <RowTable
      label={`${text(app.display_name)} micro-apps`}
      columns={
        "var(--ui-col-order) minmax(var(--ui-col-name), 2fr) minmax(var(--ui-measure-xs), 2fr) var(--ui-col-state)" +
        (manage ? " auto" : "")
      }
      empty={
        list.error
          ? list.error.message
          : list.data
            ? "No micro-apps yet."
            : "Loading…"
      }
      rows={(list.data?.items || []).map((m) => ({
        id: text(m.id),
        cells: [
          <CellMono>{text(m.navigation_order)}</CellMono>,
          <strong>{text(m.display_name)}</strong>,
          <CellMono>{text(m.artifact_ref)}</CellMono>,
          <StatusBadge state={m.state} />,
          ...(manage
            ? [
                <CellActions>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      edit(m, base + "/microapps/" + m.id, "Edit micro-app", [
                        { key: "display_name", label: "Name", required: true },
                        {
                          key: "state",
                          label: "Status",
                          type: "select",
                          options: states,
                          required: true,
                        },
                        {
                          key: "artifact_ref",
                          label: "Artifact reference",
                          advanced: true,
                        },
                        {
                          key: "navigation_order",
                          label: "Navigation order",
                          type: "number",
                          advanced: true,
                        },
                      ])
                    }
                  >
                    Edit micro-app
                  </Button>
                </CellActions>,
              ]
            : []),
        ],
      }))}
      footer={
        list.data?.pagination?.has_more && (
          <CellMono>
            Showing the first {list.data.items.length} micro-apps.
          </CellMono>
        )
      }
    />
  );
}
