import type { ReactNode } from "react";
import {
  ActivityList,
  AllClear,
  AppCard,
  AttentionCard,
  AttentionGrid,
  Button,
  PageIntro,
  Progress,
  Section,
  Skeleton,
  StatCard,
  StatGrid,
  StatusBadge,
} from "../ui";
import {
  isItem,
  text,
  type AgentGrant,
  type Deployment,
  type Page,
} from "../api-types";
import { useApiQueries, useApiQuery } from "../query";
import {
  deploymentStepLabel,
  retryDeployment,
  useDeploymentStates,
} from "../deployment";
import { agentGrantsPath, ago } from "../actors";
import { describeEvents } from "../activity";
import { createSpace } from "./forms";
import type { TenantScreen } from "./screen";

const day = 24 * 60 * 60 * 1000;

// The Tenant's Home: a greeting with the Tenant's counts, what needs a person
// now, the newest Apps and the latest activity. Every number and row comes
// from an existing management read that the backend authorizes and scopes:
// the Tenant overview (counts, newest Apps, databases being provisioned or
// failed), each listed App's latest deployment (only where this account may
// manage releases), agent grants and the audit trail. Nothing is estimated
// in the browser.
export function HomePage({ t }: { t: TenantScreen }) {
  const {
    all,
    base,
    can,
    go,
    me,
    overview,
    profile,
    scoped,
    setModal,
    tenant,
  } = t;
  const loaded = !!overview.tenant;
  const states = useDeploymentStates();
  const recent = overview.recent_apps || [];
  const deployable = recent.filter((a) =>
    scoped("release.manage", text(a.space_id), text(a.id)),
  );
  const deployments = useApiQueries<{ deployment: Deployment | null }>(
    deployable.map((a) => "/v1/apps/" + text(a.id) + "/deployment"),
  );
  const deploymentOf = new Map(
    deployable.map((a, i) => [text(a.id), deployments[i]?.data?.deployment]),
  );
  // A Tenant security manager sees every agent grant in the Tenant; anyone
  // else sees their own grants, narrowed here to this Tenant.
  const manager = all("authorization.manage");
  const grantsQuery = useApiQuery<Page<AgentGrant>>(
    agentGrantsPath(tenant, manager),
  );
  // Measured against the time the grants were read, so a render is pure.
  const now = grantsQuery.dataUpdatedAt;
  const grants = (grantsQuery.data?.items || []).filter(
    (g) =>
      g.tenant_id === tenant &&
      g.state === "active" &&
      new Date(g.expires_at).getTime() > now,
  );
  const activity = useApiQuery<Page>(
    can("audit.read") ? base + "/audit-events?limit=4" : null,
  );
  const tenantName = text(profile.display_name);

  const retry = (d: Deployment) =>
    retryDeployment(tenant, d, setModal, t.setError);

  const attention: ReactNode[] = [];
  if (loaded && overview.spaces === 0 && all("space.manage"))
    attention.push(
      <AttentionCard
        key="first-space"
        state="new"
        tone="info"
        stateLabel="Start here"
        title="Create your first Space"
        actions={
          <Button
            size="sm"
            variant="primary"
            onClick={() => void createSpace(t)}
          >
            Create Space
          </Button>
        }
      >
        A Space gives your Apps a shared database, entities and permissions.
      </AttentionCard>,
    );
  for (const app of deployable) {
    const d = deploymentOf.get(text(app.id));
    if (!isItem(d)) continue;
    const name = text(app.display_name),
      release = d.release_name || "A release";
    if (d.state === states.failed)
      attention.push(
        <AttentionCard
          key={"deploy" + d.id}
          state={d.state}
          stateLabel="Deploy failed"
          meta={name}
          title={`${release} failed to deploy`}
          actions={
            <>
              <Button size="sm" variant="primary" onClick={() => retry(d)}>
                Retry deploy
              </Button>
              <Button
                size="sm"
                onClick={() =>
                  go("app", { app: text(app.id), tab: "releases" })
                }
              >
                View timeline
              </Button>
            </>
          }
        >
          {d.error_message || "The deployment did not finish."}
        </AttentionCard>,
      );
    else if (states.running(d.state)) {
      const at = states.steps.indexOf(d.state);
      attention.push(
        <AttentionCard
          key={"deploy" + d.id}
          state={d.state}
          stateLabel="Deploying"
          tone="warning"
          meta={d.deployed_by ? `by ${d.deployed_by}` : name}
          title={`Deploying ${release} to ${name}`}
          extra={
            <Progress
              value={at + 1}
              max={states.steps.length}
              label={`Step ${at + 1} of ${states.steps.length} · ${deploymentStepLabel(d.state)}`}
            />
          }
          actions={
            <Button
              size="sm"
              onClick={() => go("app", { app: text(app.id), tab: "releases" })}
            >
              View timeline
            </Button>
          }
        >
          Production keeps serving its current release until this one is active.
        </AttentionCard>,
      );
    }
  }
  for (const db of overview.attention || []) {
    const failed = db.state === "failed";
    attention.push(
      <AttentionCard
        key={"db" + text(db.id)}
        state={text(db.state)}
        stateLabel={failed ? "Failed" : "Preparing"}
        meta="Database"
        title={
          failed
            ? `Database ${text(db.database_name)} failed`
            : `Database ${text(db.database_name)} is being prepared`
        }
        actions={
          <Button
            size="sm"
            onClick={() => go("settings", { tab: "databases" })}
          >
            Open databases
          </Button>
        }
      >
        {failed
          ? text(db.last_error_message) || "Provisioning failed."
          : "Spaces that use it can open once it is ready."}
      </AttentionCard>,
    );
  }
  for (const g of grants) {
    const left = new Date(g.expires_at).getTime() - now;
    if (left > 7 * day) continue;
    attention.push(
      <AttentionCard
        key={"grant" + g.id}
        state="expiring"
        tone="warning"
        stateLabel="Expiring"
        meta={`Agent · ${g.owner_name}`}
        title={`${g.client_name} access ends ${ago(g.expires_at)}`}
        actions={
          <Button size="sm" onClick={() => go("agents")}>
            Review agent access
          </Button>
        }
      >
        {`Scoped to ${g.scope_name}. Approve a new connection to keep it working.`}
      </AttentionCard>,
    );
  }

  const count = (value: number | undefined) =>
    loaded && value !== undefined ? value : <Skeleton width={24} />;
  return (
    <div className="ui-home">
      <PageIntro
        eyebrow={today()}
        title={`${greeting()}, ${me.display_name.split(" ")[0]}`}
        summary={
          loaded &&
          (attention.length
            ? `${attention.length} ${attention.length === 1 ? "thing needs" : "things need"} you. Everything else in ${tenantName} is healthy.`
            : `Nothing needs you right now. Everything in ${tenantName} is healthy.`)
        }
        aside={
          <StatGrid>
            {can("entity.manage") && (
              <StatCard
                label="Entities"
                value={count(overview.entities)}
                onClick={() => go("entities")}
              />
            )}
            <StatCard
              label="Apps live"
              value={count(overview.live_apps)}
              onClick={() => go("apps")}
            />
            <StatCard
              label="Agents active"
              value={grantsQuery.data ? grants.length : <Skeleton width={24} />}
              onClick={() => go("agents")}
            />
          </StatGrid>
        }
      />
      <Section title="Needs attention">
        {attention.length ? (
          <AttentionGrid>{attention}</AttentionGrid>
        ) : (
          <AllClear>
            {loaded
              ? "No failed or running deploys, databases being prepared or agent access about to expire."
              : "Checking…"}
          </AllClear>
        )}
      </Section>
      <div className="ui-home-columns">
        <Section
          className="ui-home-apps"
          title="Apps"
          actions={
            <Button variant="link" onClick={() => go("apps")}>
              All apps →
            </Button>
          }
        >
          {recent.length ? (
            <div className="ui-app-grid">
              {recent.map((a) => (
                <AppCard
                  key={text(a.id)}
                  id={text(a.id)}
                  name={text(a.display_name)}
                  subtitle={`${text(a.space_name)} · ${Number(a.microapps) || 0} ${Number(a.microapps) === 1 ? "micro-app" : "micro-apps"}`}
                  {...appDeployment(
                    text(a.state),
                    deploymentOf.get(text(a.id)),
                    deployable.some((x) => x.id === a.id),
                    states,
                  )}
                  onOpen={() => go("app", { app: text(a.id) })}
                />
              ))}
            </div>
          ) : (
            <p className="ui-home-empty">
              {loaded
                ? "No Apps yet. Apps you can open appear here."
                : "Loading…"}
            </p>
          )}
        </Section>
        {can("audit.read") && (
          <section
            className="panel ui-activity-card"
            aria-labelledby="home-activity"
          >
            <header className="ui-section-header">
              <h2 id="home-activity">Activity</h2>
              <Button variant="link" onClick={() => go("activity")}>
                Audit trail →
              </Button>
            </header>
            {activity.data?.items.length ? (
              <ActivityList
                label="Recent activity"
                items={describeEvents(
                  activity.data.items,
                  grantsQuery.data?.items || [],
                  recent,
                )}
              />
            ) : (
              <p className="ui-home-empty">
                {activity.data ? "No activity yet." : "Loading…"}
              </p>
            )}
          </section>
        )}
      </div>
    </div>
  );
}

// An App card's status pill, production address and release line, from its
// latest deployment (read only where this account may manage releases).
function appDeployment(
  appState: string,
  d: Deployment | null | undefined,
  readable: boolean,
  states: ReturnType<typeof useDeploymentStates>,
) {
  if (!readable || d === undefined)
    return { status: <StatusBadge state={appState} /> };
  if (!d)
    return {
      status: <StatusBadge state="draft" tone="neutral" label="Not deployed" />,
      footer: "No production release yet",
    };
  const release = <strong>{d.release_name || "release"}</strong>;
  const address = d.url.replace(/^https?:\/\//, "").replace(/\/$/, "");
  if (d.state === states.live)
    return {
      status: <StatusBadge state={d.state} label="Live" />,
      address,
      footer: (
        <>
          Production {release} · deployed {ago(d.finished_at || d.created_at)}
          {d.deployed_by && ` by ${d.deployed_by}`}
        </>
      ),
    };
  if (d.state === states.failed)
    return {
      status: (
        <StatusBadge
          state={d.state}
          label={`${d.release_name || "Deploy"} failed`}
        />
      ),
      address,
      footer: (
        <>
          {release} failed {ago(d.finished_at || d.created_at)} · production
          unchanged
        </>
      ),
    };
  return {
    status: <StatusBadge state={d.state} tone="warning" label="Deploying" />,
    address,
    footer: (
      <>
        Deploying {release} · started {ago(d.created_at)}
        {d.deployed_by && ` by ${d.deployed_by}`}
      </>
    ),
  };
}

// "Tuesday, 17 November": the weekday, then the date in the reader's locale.
function today() {
  const now = new Date();
  return `${now.toLocaleDateString(undefined, { weekday: "long" })}, ${now.toLocaleDateString(undefined, { day: "numeric", month: "long" })}`;
}

function greeting() {
  const hour = new Date().getHours();
  return hour < 12
    ? "Good morning"
    : hour < 18
      ? "Good afternoon"
      : "Good evening";
}
