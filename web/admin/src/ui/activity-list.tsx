import type { ReactNode } from "react";
import { cx, initials } from "./utils";
import type { AvatarKind } from "./avatar";

export type ActivityItem = {
  id: string;
  // Who acted: an agent shows "AI" on ink, the system worker a gear, a
  // person their initials.
  actor: { name: string; kind: AvatarKind };
  // One sentence built from the event ("<actor> <did> <resource>").
  text: ReactNode;
  // Who it was for or through, and when ("for <person> · <ago>").
  meta?: ReactNode;
  // ISO time for <time>.
  at?: string;
};

// A compact feed of humanized audit events: a square actor mark, one
// sentence and a muted meta line per item.
export function ActivityList({
  label,
  items,
  variant = "feed",
}: {
  label: string;
  items: ActivityItem[];
  // "rows": a full-width list (the Activity page) with the meta beside it.
  variant?: "feed" | "rows";
}) {
  return (
    <ol className={cx("ui-activity-list", "ui-" + variant)} aria-label={label}>
      {items.map((item) => (
        <li key={item.id}>
          <ActorMark {...item.actor} />
          <span className="ui-activity-copy">
            <span>{item.text}</span>
            {item.meta && (
              <span className="ui-activity-meta">
                {item.at ? (
                  <time dateTime={item.at}>{item.meta}</time>
                ) : (
                  item.meta
                )}
              </span>
            )}
          </span>
        </li>
      ))}
    </ol>
  );
}

// The square mark of an actor in a feed (decorative: the sentence names it).
export function ActorMark({ name, kind }: { name: string; kind: AvatarKind }) {
  return (
    <span className={cx("ui-actor-mark", "ui-" + kind)} aria-hidden>
      {kind === "agent" ? (
        "AI"
      ) : kind === "system" ? (
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        >
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
        </svg>
      ) : (
        initials(name)
      )}
    </span>
  );
}
