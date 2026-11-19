import {
  ActivityList,
  AllClear,
  AppCard,
  AttentionGrid,
  Button,
  PageIntro,
  Section,
  Skeleton,
  StatCard,
  StatGrid,
} from "../ui";
import { text } from "../api-types";
import { describeEvents } from "../activity";
import type { TenantScreen } from "./screen";
import { useHomeReads } from "./home-reads";
import { homeAttention } from "./home-attention";
import { appDeployment } from "./home-app-deployment";
import { greeting, today } from "./home-greeting";

// The Tenant's Home: a greeting with the Tenant's counts, what needs a person
// now, the newest Apps and the latest activity. Every number and row comes
// from an existing management read that the backend authorizes and scopes:
// the Tenant overview (counts, newest Apps, databases being provisioned or
// failed), each listed App's latest deployment (only where this account may
// manage releases), the account's own workspaces being prepared and the
// entities with unpublished changes in the sidebar's Spaces (only where it
// may manage workspaces or entities there), agent grants and the audit
// trail. Nothing is estimated in the browser.
export function HomePage({ t }: { t: TenantScreen }) {
  const { can, go, me, overview, profile } = t;
  const reads = useHomeReads(t);
  const { loaded, states, recent, deployable, deploymentOf } = reads;
  const { grantsQuery, grants, activity } = reads;
  const tenantName = text(profile.display_name);
  const attention = homeAttention(t, reads);

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
              ? "No failed or running deploys, workspaces or databases being prepared, unpublished entity changes or agent access about to expire."
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
