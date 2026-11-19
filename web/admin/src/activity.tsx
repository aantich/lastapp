import type { ReactNode } from "react";
import { isItem, text, type AgentGrant, type Item } from "./api-types";
import { cachedRows } from "./query";
import { actorOf, ago, type AgentPreset } from "./actors";
import type { ActivityItem } from "./ui";
import { family, phrases, type Name } from "./activity-phrases";

// The one place audit actions (internal/op.AuditAs callers) become sentences,
// for Home and the Activity page. An event names its own resource
// (resource_name, internal/roles/audit.go: AuditResourceName); ids in its
// details are named from the page's other events and rows this session has
// already read, else a generic noun. An action without a sentence here still
// shows, in words.

// Names of rows this session has read, by id.
function knownNames(extra: Item[]) {
  const names = new Map<string, string>();
  for (const row of [...cachedRows(() => true), ...extra]) {
    const name = text(row.display_name) || text(row.label);
    if (typeof row.id === "string" && name) names.set(row.id, name);
  }
  return names;
}

// The same actor doing the same thing to the same resource: consecutive
// events with this key read as one line.
const runKey = (e: Item) =>
  e.resource_id
    ? [
        e.actor_kind,
        e.actor_name,
        isItem(e.details_json) ? text(e.details_json.agent_grant_id) : "",
        e.action,
        e.resource_id,
      ].join("\n")
    : "";

// Audit events as activity items. `known` adds rows the caller holds outside
// list reads (the overview's recent Apps); `presets` names an agent's grant
// by its preset. With `changes`, consecutive events of one actor, action and
// resource become one line that ends with `changes(resource_id, count)` (the
// Activity page links it to that resource's trail, `?resource_id=`).
export function describeEvents(
  events: Item[],
  grants: AgentGrant[] = [],
  known: Item[] = [],
  {
    presets = [],
    changes,
  }: {
    presets?: AgentPreset[];
    changes?: (resource: string, count: number) => ReactNode;
  } = {},
): ActivityItem[] {
  const names = knownNames(known);
  for (const e of events)
    if (e.resource_id && e.resource_name)
      names.set(text(e.resource_id), text(e.resource_name));
  const n: Name = (id, noun) => {
    const name = names.get(text(id));
    return name ?? noun;
  };
  const runs: [Item, number][] = [];
  for (const e of events) {
    const last = runs[runs.length - 1];
    if (changes && last && runKey(e) && runKey(e) === runKey(last[0]))
      last[1]++;
    else runs.push([e, 1]);
  }
  return runs.map(([e, count]) => {
    const who = actorOf(e, grants, presets);
    const details = isItem(e.details_json)
      ? isItem(e.details_json.details) && e.details_json.agent_grant_id
        ? e.details_json.details
        : e.details_json
      : {};
    const phrase = phrases[text(e.action)]?.(details, n, e) ||
      family(e, n) || {
        // An action without a sentence: its words and the real resource.
        active: (
          <>
            {text(e.action).replace(/[._]/g, " ")}{" "}
            {n(e.resource_id, text(e.resource_type).replace(/_/g, " "))}
          </>
        ),
      };
    const sentence =
      who.kind === "system" && phrase.passive ? (
        phrase.passive
      ) : (
        <>
          {who.name} {phrase.active}
        </>
      );
    const when = ago(e.created_at);
    return {
      id: text(e.id),
      actor: { name: who.name, kind: who.kind },
      text:
        count > 1 && changes ? (
          <>
            {sentence} · {changes(text(e.resource_id), count)}
          </>
        ) : (
          sentence
        ),
      meta: `${who.via} · ${when}`,
      via: who.via,
      when,
      at: text(e.created_at),
    };
  });
}
