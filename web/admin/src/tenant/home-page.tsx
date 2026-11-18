import type { ReactNode } from "react";
import { CircleCheck, FolderPlus } from "lucide-react";
import {
  AttentionCard,
  AttentionGrid,
  Avatar,
  Badge,
  Button,
  Card,
  CardBody,
  CardHeader,
  Columns,
  EmptyState,
  Lead,
  RecordList,
  Section,
  Skeleton,
  StatCard,
  StatGrid,
  Stack,
  StatusBadge,
  Timeline,
} from "../ui";
import {
  isItem,
  text,
  type AgentGrant,
  type Item,
  type Page,
} from "../api-types";
import { useApiQueries, useApiQuery } from "../query";
import { useDeploymentStates } from "../deployment";
import { actorOf, agentGrantsPath, ago } from "../actors";
import { createSpace } from "./forms";
import type { TenantScreen } from "./screen";

const day = 24 * 60 * 60 * 1000;

// The Tenant's Home: what needs a person now, the Apps, agents and recent
// activity. Every number and row comes from an existing management read
// that the backend authorizes and scopes: the Tenant overview (counts,
// recent Apps, databases being provisioned or failed), each recent App's
// latest deployment (only where this account may manage releases), agent
// grants and the audit trail. Nothing is estimated in the browser.
export function HomePage({ t }: { t: TenantScreen }) {
  const { all, base, can, go, me, overview, profile, scoped, tenant } = t;
  const loaded = !!overview.tenant;
  const deploymentStates = useDeploymentStates();
  const recent = overview.recent_apps || [];
  const deployable = recent.filter((a) =>
    scoped("release.manage", text(a.space_id), text(a.id)),
  );
  const deployments = useApiQueries<{ deployment: Item | null }>(
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
    can("audit.read") ? base + "/audit-events?limit=8" : null,
  );
  const openAgents = () =>
    manager ? go("settings", { tab: "agents" }) : go("account");

  const attention: ReactNode[] = [];
  for (const db of overview.attention || []) {
    const failed = db.state === "failed";
    attention.push(
      <AttentionCard
        key={"db" + text(db.id)}
        state={text(db.state)}
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
  for (const app of deployable) {
    const d = deploymentOf.get(text(app.id));
    if (
      !isItem(d) ||
      (d.state !== deploymentStates.failed &&
        !deploymentStates.running(d.state))
    )
      continue;
    const failed = d.state === deploymentStates.failed;
    attention.push(
      <AttentionCard
        key={"deploy" + text(d.id)}
        state={text(d.state)}
        meta={ago(d.finished_at || d.created_at)}
        title={
          failed
            ? `Deploy of ${text(app.display_name)} failed`
            : `${text(app.display_name)} is deploying`
        }
        actions={
          <Button
            size="sm"
            variant={failed ? "primary" : "secondary"}
            onClick={() => go("app", { app: text(app.id), tab: "releases" })}
          >
            Open releases
          </Button>
        }
      >
        {failed
          ? text(d.error_message) || "The deployment did not finish."
          : `${text(app.space_name)} · production is updated when it finishes.`}
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
          <Button size="sm" onClick={openAgents}>
            Review agent access
          </Button>
        }
      >
        {`Scoped to ${g.scope_name}. Approve a new connection to keep it working.`}
      </AttentionCard>,
    );
  }

  const name = me.display_name.split(" ")[0];
  return (
    <Stack>
      <Lead>
        {greeting()}, {name}.{" "}
        {loaded &&
          (attention.length
            ? `${attention.length} ${attention.length === 1 ? "thing needs" : "things need"} you in ${text(profile.display_name)}.`
            : `Nothing you can act on needs attention in ${text(profile.display_name)}.`)}
      </Lead>
      <StatGrid>
        {can("space.read") && (
          <StatCard
            label="Spaces"
            value={loaded ? overview.spaces : <Skeleton width={32} />}
            onClick={() => go("spaces")}
          />
        )}
        <StatCard
          label="Apps"
          value={loaded ? overview.apps : <Skeleton width={32} />}
          detail={loaded ? `${overview.microapps ?? 0} micro-apps` : undefined}
          onClick={() => go("apps")}
        />
        {overview.databases !== undefined && (
          <StatCard
            label="Databases"
            value={overview.databases}
            onClick={() => go("settings", { tab: "databases" })}
          />
        )}
        <StatCard
          label="Active agents"
          value={grantsQuery.data ? grants.length : <Skeleton width={32} />}
          detail={manager ? "In this Tenant" : "Connected by you"}
          onClick={openAgents}
        />
      </StatGrid>
      <Columns
        tracks={[
          { grow: 8, min: 480 },
          { grow: 5, min: 320 },
        ]}
      >
        <Stack>
          <Section title="Needs attention">
            {overview.spaces === 0 && all("space.manage") ? (
              <Card>
                <EmptyState
                  icon={<FolderPlus size={28} aria-hidden />}
                  title="Create your first Space"
                  actions={
                    <Button
                      variant="primary"
                      onClick={() => void createSpace(t)}
                    >
                      Create Space
                    </Button>
                  }
                >
                  A Space gives your Apps a shared database.
                </EmptyState>
              </Card>
            ) : attention.length ? (
              <AttentionGrid>{attention}</AttentionGrid>
            ) : (
              <Card>
                <EmptyState
                  compact
                  icon={<CircleCheck size={24} aria-hidden />}
                  title={loaded ? "All clear" : "Checking…"}
                >
                  Failed or running deployments, databases being prepared and
                  agent access about to expire appear here.
                </EmptyState>
              </Card>
            )}
          </Section>
          <Card>
            <CardHeader
              title="Apps"
              description="Most recently created"
              actions={
                <Button variant="link" onClick={() => go("apps")}>
                  All Apps
                </Button>
              }
            />
            <CardBody>
              {recent.length ? (
                <RecordList
                  label="Recent Apps"
                  rows={recent.map((a) => {
                    const d = deploymentOf.get(text(a.id));
                    return {
                      id: text(a.id),
                      leading: <Avatar name={text(a.display_name)} size={34} />,
                      title: text(a.display_name),
                      meta: text(a.space_name),
                      detail: isItem(d) && (
                        <StatusBadge
                          state={text(d.state)}
                          label={`Deploy ${text(d.state)}`}
                        />
                      ),
                      trailing: (
                        <>
                          <StatusBadge state={text(a.state)} />
                          <Button
                            size="sm"
                            variant="ghost"
                            aria-label={`Open ${text(a.display_name)}`}
                            onClick={() => go("app", { app: text(a.id) })}
                          >
                            Open
                          </Button>
                        </>
                      ),
                    };
                  })}
                />
              ) : (
                <EmptyState compact title={loaded ? "No Apps yet" : "Loading…"}>
                  {loaded && "Apps you can open appear here."}
                </EmptyState>
              )}
            </CardBody>
          </Card>
        </Stack>
        <Stack>
          {can("audit.read") && (
            <Card>
              <CardHeader
                title="Recent activity"
                description="People, agents and the system worker"
                actions={
                  <Button variant="link" onClick={() => go("activity")}>
                    Full audit trail
                  </Button>
                }
              />
              <CardBody>
                {activity.data?.items.length ? (
                  <Timeline
                    label="Recent activity"
                    events={activity.data.items.map((e) => {
                      const actor = actorOf(e, grantsQuery.data?.items || []);
                      return {
                        id: text(e.id),
                        marker: (
                          <Avatar
                            name={actor.name}
                            kind={actor.kind}
                            size={28}
                          />
                        ),
                        title: actor.label,
                        meta: <Badge mono>{text(e.action)}</Badge>,
                        detail: text(e.resource_type),
                        at: text(e.created_at),
                        when: ago(e.created_at),
                      };
                    })}
                  />
                ) : (
                  <EmptyState compact>
                    {activity.data ? "No activity yet." : "Loading…"}
                  </EmptyState>
                )}
              </CardBody>
            </Card>
          )}
          <Card>
            <CardHeader
              title="Agents"
              description={
                manager ? "Connected in this Tenant" : "Connected by you"
              }
              actions={
                <Button variant="link" onClick={openAgents}>
                  Agent access
                </Button>
              }
            />
            <CardBody>
              {grants.length ? (
                <RecordList
                  label="Connected agents"
                  rows={grants.slice(0, 5).map((g) => ({
                    id: g.id,
                    leading: (
                      <Avatar name={g.client_name} kind="agent" size={34} />
                    ),
                    title: g.client_name,
                    meta: `for ${g.owner_name} · ${g.scope_name}`,
                    trailing: (
                      <small>
                        {g.last_used_at
                          ? `Used ${ago(g.last_used_at)}`
                          : "Not used yet"}
                      </small>
                    ),
                  }))}
                />
              ) : (
                <EmptyState compact>
                  {grantsQuery.data
                    ? manager
                      ? "No agent is connected to this Tenant."
                      : "You have not connected an agent. Connect one from My account → Agent access."
                    : "Loading…"}
                </EmptyState>
              )}
            </CardBody>
          </Card>
        </Stack>
      </Columns>
    </Stack>
  );
}

function greeting() {
  const hour = new Date().getHours();
  return hour < 12
    ? "Good morning"
    : hour < 18
      ? "Good afternoon"
      : "Good evening";
}
