import type { ReactNode } from "react";
import { isItem, optText, text, type Item } from "./api-types";

// The sentence of each audit action, for describeEvents (activity.tsx).

type Details = Record<string, unknown>;
export type Name = (id: unknown, noun: string) => ReactNode;
// `active` follows the actor ("<actor> deployed …"); `passive` is a whole
// sentence for the system worker ("Deploy of … failed").
type Phrase = (
  d: Details,
  n: Name,
  e: Item,
) => {
  active: ReactNode;
  passive?: ReactNode;
};

const mono = (v: unknown) => <code>{text(v)}</code>;
const state = (e: Item) => text(e.action).split(".").slice(1).join(" ");

export const phrases: Record<string, Phrase> = {
  "space.created": (_, n, e) => ({
    active: <>created Space {n(e.resource_id, "a Space")}</>,
  }),
  "app.created": (d, n, e) => ({
    active: (
      <>
        created App {n(e.resource_id, "an App")} in {n(d.space_id, "a Space")}
      </>
    ),
  }),
  "microapp.created": (d, n) => ({
    active: <>added a micro-app to {n(d.app_id, "an App")}</>,
  }),
  "tenant.created": () => ({ active: "created the Tenant" }),
  "tenant.updated": () => ({ active: "updated the Tenant" }),
  "tenant.profile_updated": (d) => ({
    active: <>renamed the Tenant to {text(d.display_name)}</>,
  }),
  "plan.created": (d) => ({ active: <>created plan {mono(d.code)}</> }),
  "plan.updated": () => ({ active: "updated a plan" }),
  "build.uploaded": (_, n, e) => ({
    active: <>uploaded a build of {n(e.resource_id, "an App")}</>,
  }),
  "release.created": (_, n, e) => ({
    active: <>created release {n(e.resource_id, "of an App")}</>,
  }),
  "release.preview_activated": (d, n, e) => ({
    active: (
      <>
        previewed {n(d.release_id, "a release")} in {n(e.resource_id, "an App")}
      </>
    ),
  }),
  "release.deployment_queued": (_, n, e) => ({
    active: <>started a deploy of {n(e.resource_id, "a release")}</>,
  }),
  "release.deployed": (_, n, e) => ({
    active: <>deployed {n(e.resource_id, "a release")}</>,
    passive: <>{n(e.resource_id, "A release")} is live</>,
  }),
  "release.deployment_failed": (_, n, e) => ({
    active: <>could not deploy {n(e.resource_id, "a release")}</>,
    passive: <>Deploy of {n(e.resource_id, "a release")} failed</>,
  }),
  "workspace.created": (_, n, e) => ({
    active: <>started workspace {n(e.resource_id, "for development")}</>,
  }),
  "entity.created": (_, n, e) => ({
    active: <>created entity {n(e.resource_id, "an entity")}</>,
  }),
  "entity.saved": (_, n, e) => ({
    active: <>saved a draft of {n(e.resource_id, "an entity")}</>,
  }),
  "entity.published": (_, n, e) => ({
    active: <>published entity {n(e.resource_id, "changes")}</>,
    passive: <>Entity {n(e.resource_id, "changes")} published</>,
  }),
  "entity.publication_failed": (_, n, e) => ({
    active: <>could not publish {n(e.resource_id, "an entity")}</>,
    passive: <>Publishing {n(e.resource_id, "an entity")} failed</>,
  }),
  "app.data_access_changed": (_, n, e) => ({
    active: <>changed data access of {n(e.resource_id, "an App")}</>,
  }),
  "database.requested": (d) => ({
    active: <>requested a {text(d.layout) || "new"} database</>,
  }),
  "database.schema_credentials_changed": () => ({
    active: "rotated a database's schema credentials",
  }),
  "target.created": () => ({ active: "added a database server" }),
  "target.updated": () => ({ active: "updated a database server" }),
  "agent.approved": (d) => ({
    active: <>connected agent {mono(d.client_id)}</>,
  }),
  "agent.changed": (d) => ({
    active: <>changed an agent connection ({text(d.state) || "updated"})</>,
  }),
  "role.created": (d) => ({ active: <>created role {roleName(d)}</> }),
  "role.updated": (d) => ({ active: <>updated role {roleName(d)}</> }),
  "grant.created": () => ({ active: "assigned a role" }),
  "grant.deleted": () => ({ active: "removed a role assignment" }),
  "membership.created": (_, n, e) => ({
    active: <>added {n(e.resource_id, "a member")} to the Tenant</>,
  }),
  "membership.updated": (_, n, e) => ({
    active: <>updated the membership of {n(e.resource_id, "a member")}</>,
  }),
  "group.created": (_, n, e) => ({
    active: <>created team {n(e.resource_id, "a team")}</>,
  }),
  "group.updated": (_, n, e) => ({
    active: <>updated team {n(e.resource_id, "a team")}</>,
  }),
  "group.members_changed": (_, n, e) => ({
    active: <>changed the members of {n(e.resource_id, "a team")}</>,
  }),
  "team.membership.updated": (_, n, e) => ({
    active: <>changed the members of {n(e.resource_id, "a team")}</>,
  }),
  "team.roles.revoked_on_leave": () => ({
    active: "revoked team roles after a member left",
  }),
  "user.created": (d) => ({
    active: <>created account {text(d.display_name) || "for a person"}</>,
  }),
  "user.updated": (_, n, e) => ({
    active: <>updated account {n(e.resource_id, "of a person")}</>,
  }),
  "user.password_changed": () => ({ active: "changed a password" }),
  "platform.bootstrap": () => ({ active: "set up the platform" }),
  "development.seeded": () => ({ active: "seeded the development demo" }),
};

// Families named by a state suffix: "database.ready", "workspace.failed",
// "<table>.updated" (an inventory state change).
export function family(e: Item, n: Name): ReturnType<Phrase> | undefined {
  const [kind] = text(e.action).split(".");
  const now = state(e);
  if (kind === "database")
    return {
      active: <>marked a database {now}</>,
      passive: <>A database is {now}</>,
    };
  if (kind === "workspace")
    return {
      active: (
        <>
          marked workspace {n(e.resource_id, "for development")} {now}
        </>
      ),
      passive: (
        <>
          Workspace {n(e.resource_id, "for development")} is {now}
        </>
      ),
    };
  if (now === "updated") {
    const noun = text(e.resource_type).replace(/_/g, " ");
    const s = isItem(e.details_json) ? optText(e.details_json.state) : "";
    return {
      active: (
        <>
          {s === "archived" ? "archived" : "updated"} {noun}{" "}
          {n(e.resource_id, "")}
        </>
      ),
    };
  }
  return undefined;
}

function roleName(d: Details) {
  const after = isItem(d.after) ? d.after : isItem(d.before) ? d.before : {};
  return text(after.display_name) || "a role";
}
