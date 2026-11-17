import {
  forwardRef,
  useEffect,
  useId,
  useImperativeHandle,
  useRef,
  type InputHTMLAttributes,
  type ReactNode,
} from "react";
import { cx } from "./utils";

type CheckboxProps = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "type" | "onChange" | "checked"
> & {
  checked: boolean;
  onChange: (checked: boolean) => void;
  // Mixed state (some children checked); announced as "mixed".
  indeterminate?: boolean;
  // Visible label; omit only with an aria-label (e.g. a matrix cell).
  label?: ReactNode;
  description?: ReactNode;
};

// A native checkbox with the Signal face (17px, 3:1 boundary): Space toggles
// it, forms and getByRole("checkbox") see a real checkbox.
export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  function Checkbox(
    {
      checked,
      onChange,
      indeterminate = false,
      label,
      description,
      className,
      ...rest
    },
    forwarded,
  ) {
    const ref = useRef<HTMLInputElement>(null);
    useImperativeHandle(forwarded, () => ref.current as HTMLInputElement);
    useEffect(() => {
      if (ref.current) ref.current.indeterminate = indeterminate;
    }, [indeterminate]);
    const input = (
      <input
        ref={ref}
        type="checkbox"
        className="ui-checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        {...rest}
      />
    );
    if (!label) return input;
    return (
      <label className={cx("ui-check", className)}>
        {input}
        <span className="ui-check-copy">
          <span>{label}</span>
          {description && <small>{description}</small>}
        </span>
      </label>
    );
  },
);

// An on/off setting that applies immediately (role=switch, Space/Enter).
// Use a Checkbox for a choice that is saved with a form.
export function Switch({
  checked,
  onChange,
  label,
  description,
  disabled,
  id,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: ReactNode;
  description?: ReactNode;
  disabled?: boolean;
  id?: string;
}) {
  const generated = useId(),
    switchId = id || generated;
  return (
    <span className="ui-switch-row">
      <button
        type="button"
        role="switch"
        id={switchId}
        className="ui-switch"
        aria-checked={checked}
        aria-labelledby={`${switchId}-label`}
        aria-describedby={description ? `${switchId}-description` : undefined}
        disabled={disabled}
        onClick={() => onChange(!checked)}
      />
      <span className="ui-check-copy">
        <label id={`${switchId}-label`} htmlFor={switchId}>
          {label}
        </label>
        {description && (
          <small id={`${switchId}-description`}>{description}</small>
        )}
      </span>
    </span>
  );
}

export type RadioOption = {
  value: string;
  label: ReactNode;
  description?: ReactNode;
  disabled?: boolean;
};

// One choice among a few visible options: a fieldset of native radios, so
// arrow keys move and select within the group.
export function RadioGroup({
  legend,
  name,
  options,
  value,
  onChange,
  horizontal,
}: {
  legend: ReactNode;
  name?: string;
  options: RadioOption[];
  value: string;
  onChange: (value: string) => void;
  horizontal?: boolean;
}) {
  const generated = useId();
  return (
    <fieldset className={cx("ui-radio-group", horizontal && "ui-horizontal")}>
      <legend>{legend}</legend>
      <div>
        {options.map((option) => (
          <label key={option.value} className="ui-radio">
            <input
              type="radio"
              className="ui-radio-input"
              name={name || generated}
              value={option.value}
              checked={value === option.value}
              disabled={option.disabled}
              onChange={() => onChange(option.value)}
            />
            <span className="ui-check-copy">
              <span>{option.label}</span>
              {option.description && <small>{option.description}</small>}
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}
