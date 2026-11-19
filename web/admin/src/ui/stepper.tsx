import { useId, type CSSProperties, type ReactNode } from "react";
import { Check, CircleDot, Circle, X } from "lucide-react";
import { cx } from "./utils";
import { measure, type Measure } from "./measure";
import { Icon } from "./icon";

export type StepState = "done" | "active" | "pending" | "failed";
export type Step = {
  id: string;
  label: ReactNode;
  note?: ReactNode;
  state: StepState;
};

const marks = { done: Check, active: CircleDot, pending: Circle, failed: X };
const spoken: Record<StepState, string> = {
  done: "done",
  active: "in progress",
  pending: "not started",
  failed: "failed",
};

// A pipeline such as deploy (Queued → Preparing storage → Publishing →
// Active) or publish (Drafted → Validated → Review → Prepare storage →
// Published). Each step's state is spoken as text, the active step is
// aria-current="step"; the mark shape repeats the state for sighted users.
export function Stepper({
  steps,
  label,
  min,
}: {
  steps: Step[];
  label: string;
  // The narrowest a step gets before the row wraps (default "2xs").
  min?: Measure;
}) {
  return (
    <ol
      className="ui-stepper"
      aria-label={label}
      style={
        min ? ({ "--ui-step-min": measure(min) } as CSSProperties) : undefined
      }
    >
      {steps.map((step) => {
        const Mark = marks[step.state];
        return (
          <li
            key={step.id}
            className="ui-step"
            data-state={step.state}
            aria-current={step.state === "active" ? "step" : undefined}
          >
            <span className="ui-step-label">
              <Icon icon={Mark} size="sm" aria-hidden stroke="bold" />
              {step.label}
              <span className="ui-sr-only">, {spoken[step.state]}</span>
            </span>
            {step.note && <span className="ui-step-note">{step.note}</span>}
          </li>
        );
      })}
    </ol>
  );
}

// Determinate progress ("Step 3 of 4 · Publishing").
export function Progress({
  value,
  max = 100,
  label,
}: {
  value: number;
  max?: number;
  // Names the progressbar and is shown under it.
  label: ReactNode;
}) {
  const id = useId();
  const percent = Math.max(0, Math.min(100, (value / max) * 100));
  return (
    <div className="ui-progress">
      <div
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={value}
        aria-valuetext={typeof label === "string" ? label : undefined}
        aria-labelledby={id}
      >
        <span style={{ width: `${percent}%` }} />
      </div>
      <span id={id}>{label}</span>
    </div>
  );
}

export type TimelineEvent = {
  id: string;
  // Usually an Avatar of the human or agent who acted.
  marker: ReactNode;
  title: ReactNode;
  detail?: ReactNode;
  // Machine-readable time for <time dateTime> (its full local time is the
  // hover title); `when` is the visible text.
  at?: string;
  when?: ReactNode;
  meta?: ReactNode;
};

// A vertical history (audit entries, deployment attempts), newest first.
export function Timeline({
  events,
  label,
  className,
}: {
  events: TimelineEvent[];
  label: string;
  className?: string;
}) {
  return (
    <ol className={cx("ui-timeline", className)} aria-label={label}>
      {events.map((event) => (
        <li key={event.id}>
          {event.marker}
          <div className="ui-timeline-copy">
            <div>
              <strong>{event.title}</strong>
              {event.meta}
            </div>
            {event.detail && <span>{event.detail}</span>}
            {event.when && (
              <time
                dateTime={event.at}
                title={
                  event.at ? new Date(event.at).toLocaleString() : undefined
                }
              >
                {event.when}
              </time>
            )}
          </div>
        </li>
      ))}
    </ol>
  );
}
