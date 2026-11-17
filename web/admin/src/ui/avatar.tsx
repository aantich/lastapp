import { Bot, Settings } from "lucide-react";
import { cx, initials } from "./utils";

export type AvatarKind = "person" | "agent" | "system";

// A person (initials), an agent (bot mark on ink) or the system worker.
// Decorative by default because the name is printed beside it; pass
// `labelled` when the avatar stands alone.
export function Avatar({
  name,
  kind = "person",
  size = 32,
  labelled,
  className,
}: {
  name: string;
  kind?: AvatarKind;
  size?: number;
  labelled?: boolean;
  className?: string;
}) {
  const label =
    kind === "agent"
      ? `${name} (agent)`
      : kind === "system"
        ? `${name} (system)`
        : name;
  return (
    <span
      className={cx(
        "ui-avatar",
        kind === "agent" && "ui-agent",
        kind === "system" && "ui-system",
        className,
      )}
      style={{ "--ui-avatar-size": `${size}px` } as React.CSSProperties}
      role={labelled ? "img" : undefined}
      aria-label={labelled ? label : undefined}
      aria-hidden={labelled ? undefined : true}
      title={labelled ? label : undefined}
    >
      {kind === "agent" ? (
        <Bot size={Math.round(size * 0.5)} aria-hidden />
      ) : kind === "system" ? (
        <Settings size={Math.round(size * 0.5)} aria-hidden />
      ) : (
        initials(name)
      )}
    </span>
  );
}
