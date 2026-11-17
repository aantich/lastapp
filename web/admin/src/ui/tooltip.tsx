import {
  cloneElement,
  useEffect,
  useId,
  useState,
  type ReactElement,
  type ReactNode,
} from "react";

type TooltipProps = {
  content: ReactNode;
  side?: "top" | "right" | "bottom";
  // The content repeats the trigger's accessible name (an icon button's
  // label): it is shown, but not announced a second time.
  labelOnly?: boolean;
  // Suppress the tooltip (e.g. a rail item whose label is visible).
  disabled?: boolean;
  children: ReactElement<{ "aria-describedby"?: string }>;
};

// A short label shown on pointer hover and keyboard focus; Escape hides it.
// Never put interactive content in a tooltip.
export function Tooltip({
  content,
  side = "top",
  labelOnly,
  disabled,
  children,
}: TooltipProps) {
  const id = useId(),
    [open, setOpen] = useState(false);
  useEffect(() => {
    if (!open) return;
    const escape = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", escape);
    return () => document.removeEventListener("keydown", escape);
  }, [open]);
  const show = open && !disabled;
  return (
    <span
      className="ui-tooltip-anchor"
      onPointerEnter={() => setOpen(true)}
      onPointerLeave={() => setOpen(false)}
      onFocus={(e) => setOpen(e.target.matches(":focus-visible"))}
      onBlur={() => setOpen(false)}
    >
      {labelOnly || disabled
        ? children
        : cloneElement(children, { "aria-describedby": show ? id : undefined })}
      {show && (
        <span
          className="ui-tooltip"
          data-side={side}
          id={id}
          role={labelOnly ? undefined : "tooltip"}
          aria-hidden={labelOnly || undefined}
        >
          {content}
        </span>
      )}
    </span>
  );
}
