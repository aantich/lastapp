import { Bot, Settings } from "lucide-react";
import type { CSSProperties } from "react";
import { cx, initials } from "./utils";

export type AvatarKind = "person" | "agent" | "system";
// The --ui-avatar-* tokens (src/theme/sizes.css).
export type AvatarSize = "xs" | "sm" | "md" | "lg" | "xl";

// A person (initials), an agent (bot mark on ink) or the system worker.
// Decorative by default because the name is printed beside it; pass
// `labelled` when the avatar stands alone.
export function Avatar({
  name,
  kind = "person",
  size = "md",
  labelled,
  className,
}: {
  name: string;
  kind?: AvatarKind;
  size?: AvatarSize;
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
      style={
        { "--ui-avatar-size": `var(--ui-avatar-${size})` } as CSSProperties
      }
      role={labelled ? "img" : undefined}
      aria-label={labelled ? label : undefined}
      aria-hidden={labelled ? undefined : true}
      title={labelled ? label : undefined}
    >
      {kind === "agent" ? (
        <Bot aria-hidden />
      ) : kind === "system" ? (
        <Settings aria-hidden />
      ) : (
        initials(name)
      )}
    </span>
  );
}
