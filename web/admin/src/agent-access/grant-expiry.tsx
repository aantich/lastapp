import {
  addDays,
  DatePicker,
  daysBetween,
  FlowSection,
  FormField,
  isoDay,
  parseDay,
  SegmentedControl,
} from "../ui";
import type { AgentConfig } from "../api-types";
import { dayLabel, type GrantValuesProps } from "./grant-values";

// When access ends, as a number of days from today.
export function GrantExpiry({
  config,
  values,
  setValues,
}: GrantValuesProps & { config: AgentConfig }) {
  const today = new Date(),
    expiry = isoDay(addDays(today, values.days)),
    quick = (config.day_choices || []).filter(
      (d) => d >= 1 && d <= config.max_days,
    );
  return (
    <FlowSection label="Expires after">
      {quick.length > 0 && (
        <SegmentedControl
          label="Access length"
          options={quick.map((d) => ({
            value: String(d),
            label: dayLabel(d),
          }))}
          value={String(values.days)}
          onChange={(d) => setValues({ ...values, days: Number(d) })}
        />
      )}
      <FormField
        label="Access expires on"
        hint={`${dayLabel(values.days)} from now; at most ${config.max_days}. Closing the browser or signing out does not end it.`}
      >
        <DatePicker
          value={expiry}
          min={isoDay(addDays(today, 1))}
          max={isoDay(addDays(today, config.max_days))}
          onChange={(day) => {
            const chosen = parseDay(day);
            if (chosen)
              setValues({ ...values, days: daysBetween(today, chosen) });
          }}
        />
      </FormField>
    </FlowSection>
  );
}
