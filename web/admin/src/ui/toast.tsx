import { useEffect, useState, useSyncExternalStore } from "react";
import { CircleAlert, CircleCheck, Info, X } from "lucide-react";
import {
  dismissToast,
  subscribeToasts,
  currentToasts,
  type Toast,
} from "./toast-store";
import { Icon } from "./icon";

const icons = { info: Info, success: CircleCheck, danger: CircleAlert };
// How long a toast stays unless it is held (ms).
const toastDuration = 5000;

function ToastItem({ item }: { item: Toast }) {
  const { tone = "info", duration = toastDuration } = item;
  const [paused, setPaused] = useState(false);
  const hold = tone === "danger" || duration === 0;
  useEffect(() => {
    if (hold || paused) return;
    const timer = setTimeout(() => dismissToast(item.id), duration);
    return () => clearTimeout(timer);
  }, [item.id, hold, paused, duration]);
  const Glyph = icons[tone];
  return (
    <div
      className="ui-toast"
      data-tone={tone}
      onPointerEnter={() => setPaused(true)}
      onPointerLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
    >
      <Icon icon={Glyph} size="lg" aria-hidden />
      <div>
        {item.message}
        {item.action && (
          <button
            type="button"
            className="text-button"
            onClick={() => {
              item.action?.onClick();
              dismissToast(item.id);
            }}
          >
            {item.action.label}
          </button>
        )}
      </div>
      <button
        type="button"
        className="icon"
        aria-label="Dismiss notification"
        onClick={() => dismissToast(item.id)}
      >
        <Icon icon={X} size="md" aria-hidden />
      </button>
    </div>
  );
}

// Mounted once at the console root (main.tsx). The live regions (aria-live
// without a role, so they never count as page alerts) exist before
// any toast so screen readers announce additions: polite for info/success,
// assertive for danger. Toasts pause while hovered or focused.
export function Toaster() {
  const items = useSyncExternalStore(subscribeToasts, currentToasts);
  return (
    <div className="ui-toaster">
      <div aria-live="polite" className="ui-toaster-region">
        {items
          .filter((t) => t.tone !== "danger")
          .map((t) => (
            <ToastItem key={t.id} item={t} />
          ))}
      </div>
      <div aria-live="assertive" className="ui-toaster-region">
        {items
          .filter((t) => t.tone === "danger")
          .map((t) => (
            <ToastItem key={t.id} item={t} />
          ))}
      </div>
    </div>
  );
}
