import {
  AppWindow,
  ArrowUpRight,
  Building2,
  CalendarDays,
  ChevronRight,
  Database,
  FolderOpen,
  Layers3,
  RefreshCw,
  Server,
} from "lucide-react";

type Item = Record<string, any>;
const resources = [
  {
    key: "tenants",
    label: "Tenants",
    Icon: Building2,
    page: "tenants",
    note: "Company workspaces",
  },
  {
    key: "spaces",
    label: "Spaces",
    Icon: FolderOpen,
    page: "spaces",
    note: "Business data & rules",
  },
  {
    key: "apps",
    label: "Apps",
    Icon: AppWindow,
    page: "apps",
    note: "Registered applications",
  },
  {
    key: "database_resources",
    label: "Databases",
    Icon: Database,
    page: "databases",
    note: "Managed & connected",
  },
];
const databaseStates = [
  { state: "ready", label: "Ready", color: "var(--ui-chart-primary)" },
  { state: "pending", label: "Pending", color: "var(--ui-chart-warning)" },
  {
    state: "provisioning",
    label: "Provisioning",
    color: "var(--ui-chart-success)",
  },
  { state: "failed", label: "Failed", color: "var(--ui-chart-danger)" },
  { state: "other", label: "Other", color: "var(--ui-faint)" },
];
function Badge({ state }: { state: string }) {
  return (
    <span className={"badge " + state}>
      <i />
      {state.replaceAll("_", " ")}
    </span>
  );
}

export function OverviewDashboard({
  overview,
  name,
  navigate,
  refresh,
}: {
  overview: Item;
  name: string;
  navigate: (page: string) => void;
  refresh: () => void;
}) {
  const states = overview.database_states || [];
  const total = states.reduce(
    (sum: number, item: Item) => sum + Number(item.count),
    0,
  );
  const segments = databaseStates.map((item) => ({
    ...item,
    count:
      item.state === "other"
        ? states
            .filter(
              (s: Item) => !databaseStates.some((v) => v.state === s.state),
            )
            .reduce((sum: number, s: Item) => sum + Number(s.count), 0)
        : Number(states.find((s: Item) => s.state === item.state)?.count || 0),
  }));
  let offset = 0;
  const max = Math.max(
    1,
    ...resources.map((item) => Number(overview[item.key] || 0)),
  );
  return (
    <>
      <div className="welcome-banner">
        <div>
          <span className="welcome-symbol">
            <Layers3 size={22} />
          </span>
          <div>
            <h2>Good day, {name}!</h2>
            <p>Here’s what’s happening across your platform.</p>
          </div>
        </div>
        <span className="dashboard-date">
          <CalendarDays size={15} />
          {new Date().toLocaleDateString(undefined, {
            month: "short",
            day: "numeric",
            year: "numeric",
          })}
        </span>
      </div>
      <div className="dashboard-grid">
        <div className="stats">
          {resources.map(({ key, label, Icon, page, note }) => (
            <button
              className="stat"
              key={key}
              onClick={() => navigate(page)}
              aria-label={"View " + label.toLowerCase()}
            >
              <span>
                <span>{label}</span>
                <i className={"stat-icon tone-" + key}>
                  <Icon size={21} />
                </i>
              </span>
              <strong>{overview[key] ?? "—"}</strong>
              <small>
                {note}
                <ArrowUpRight size={14} />
              </small>
            </button>
          ))}
        </div>
        <section className="panel health">
          <div className="panel-heading">
            <h2>Database health</h2>
            <button
              className="icon"
              aria-label="Refresh database health"
              onClick={refresh}
            >
              <RefreshCw size={15} />
            </button>
          </div>
          <div className="donut-wrap">
            <svg
              viewBox="0 0 180 180"
              className="donut"
              role="img"
              aria-label={
                total +
                " databases: " +
                segments
                  .map((s) => `${s.count} ${s.label.toLowerCase()}`)
                  .join(", ")
              }
            >
              <circle className="donut-track" cx="90" cy="90" r="70" />
              {segments.map((segment) => {
                const length = total ? (segment.count / total) * 100 : 0;
                const start = offset;
                offset += length;
                return (
                  <circle
                    key={segment.state}
                    cx="90"
                    cy="90"
                    r="70"
                    pathLength="100"
                    fill="none"
                    stroke={segment.color}
                    strokeWidth="20"
                    strokeDasharray={`${length} ${100 - length}`}
                    strokeDashoffset={-start}
                    transform="rotate(-90 90 90)"
                  />
                );
              })}
              <text x="90" y="81" className="donut-label">
                Total databases
              </text>
              <text x="90" y="111" className="donut-value">
                {total}
              </text>
            </svg>
          </div>
          <div className="health-legend">
            {segments
              .filter((s) => s.state !== "other" || s.count)
              .map((s) => (
                <div key={s.state}>
                  <span>
                    <i style={{ background: s.color }} />
                    {s.label}
                  </span>
                  <strong>{s.count}</strong>
                </div>
              ))}
          </div>
        </section>
        <section className="panel resource-distribution">
          <div className="panel-heading">
            <h2>Resource distribution</h2>
            <Server size={16} />
          </div>
          <p className="chart-caption">Current inventory across the platform</p>
          <div className="resource-bars">
            {resources.map(({ key, label, Icon }) => (
              <div className="resource-bar" key={key}>
                <div>
                  <span>
                    <Icon size={14} />
                    {label}
                  </span>
                  <strong>{overview[key] ?? "—"}</strong>
                </div>
                <div className="bar-track">
                  <span
                    className={"tone-" + key}
                    style={{
                      width: `${(Number(overview[key] || 0) / max) * 100}%`,
                    }}
                  />
                </div>
              </div>
            ))}
          </div>
          <div className="chart-note">
            <span className="live-dot" />
            Inventory updates every 5 seconds
          </div>
        </section>
      </div>
      <div className="overview-grid">
        <section className="panel recent-resources">
          <div className="panel-heading">
            <div>
              <h2>Recent database resources</h2>
              <p>Provisioning status across the platform</p>
            </div>
            <span className="tag">
              <span className="live-dot" />
              Live updates
            </span>
          </div>
          {overview.recent_databases?.length ? (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Database</th>
                    <th>Tenant</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {overview.recent_databases.map((d: Item) => (
                    <tr key={d.id}>
                      <td>
                        <div className="entity">
                          <span className="resource-glyph">
                            <Database size={16} />
                          </span>
                          <code>{d.database_name}</code>
                        </div>
                      </td>
                      <td>{d.tenant_name}</td>
                      <td>
                        <Badge state={d.state} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="empty compact">
              <Database size={28} />
              <h3>No databases yet</h3>
              <p>Create your first Tenant and Space to provision one.</p>
            </div>
          )}
          <div className="table-footer">
            <span>Latest resources</span>
            <button className="text-button" onClick={() => navigate("tenants")}>
              Manage Tenants
              <ArrowUpRight size={13} />
            </button>
          </div>
        </section>
        <section className="panel quick-start">
          <div className="panel-heading">
            <h2>Your next building block</h2>
            <span className="tag accent">Getting started</span>
          </div>
          <p>
            Start with a Tenant. Give it a Space, connect a database, then
            organize the Apps your teams will use.
          </p>
          <div className="hierarchy">
            {[
              { Icon: Building2, title: "Tenant", note: "The company" },
              { Icon: FolderOpen, title: "Space", note: "Data & rules" },
              { Icon: AppWindow, title: "App", note: "API & clients" },
            ].map(({ Icon, title, note }, i) => (
              <div className="hierarchy-step" key={title}>
                <span className="resource-glyph">
                  <Icon size={18} />
                </span>
                <div>
                  <b>{title}</b>
                  <small>{note}</small>
                </div>
                {i < 2 && <ChevronRight size={15} />}
              </div>
            ))}
          </div>
          <button className="secondary" onClick={() => navigate("tenants")}>
            Manage your Tenants
            <ArrowUpRight size={14} />
          </button>
        </section>
      </div>
    </>
  );
}
