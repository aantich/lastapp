import { RemoteSelect } from "./remote-select";
import { Select } from "./select";
import { NumberInput } from "./number-input";
import { Checkbox } from "./checkbox";
import {
  FormModalTextInput,
  type FieldInputProps,
} from "./form-modal-text-input";
import { keyOf, text } from "../api-types";
// A multi-value field's current list; unset means none chosen.
const listValue = (v: unknown): unknown[] => (Array.isArray(v) ? v : []);

// One FormModal field: its label, the control its type selects and its hint.
export function FormModalField(
  props: FieldInputProps & { lookups: Record<string, string> },
) {
  const { f, autoFocus, values, setValues, policy, lookups } = props;
  const Container =
    f.type === "permissions" || f.type === "note" ? "div" : "label";
  return (
    <Container
      className={
        "form-field" +
        (f.wide ||
        f.newPassword ||
        f.type === "permissions" ||
        f.type === "note"
          ? " wide"
          : "")
      }
    >
      {f.type !== "checkbox" && (
        <span className="field-label">
          {f.label}
          {f.required && <b aria-hidden="true"> *</b>}
        </span>
      )}
      {f.type === "note" ? (
        <p className="hint">{f.hint}</p>
      ) : f.type === "checkbox" ? (
        <span className="check-label">
          <Checkbox
            checked={!!values[f.key]}
            onChange={(checked) => setValues({ ...values, [f.key]: checked })}
          />
          {f.label}
        </span>
      ) : f.type === "permissions" ? (
        <div className="permission-list">
          {f.options?.map((o) => (
            <label key={keyOf(o.value)}>
              <Checkbox
                checked={listValue(values[f.key]).includes(o.value)}
                onChange={(checked) =>
                  setValues({
                    ...values,
                    [f.key]: checked
                      ? [...listValue(values[f.key]), o.value]
                      : listValue(values[f.key]).filter((x) => x !== o.value),
                  })
                }
              />
              <span>{text(o.label)}</span>
            </label>
          ))}
        </div>
      ) : f.type === "select" && (f.lookup || lookups[f.key]) ? (
        <RemoteSelect
          label={f.label}
          required={f.required}
          value={String(values[f.key] ?? "")}
          initial={(f.options || []).map((o) => ({
            value: text(o.value),
            label: text(o.label),
          }))}
          path={f.lookup || lookups[f.key]}
          valueKey={
            (f.lookup || lookups[f.key]).endsWith("memberships") ||
            (f.lookup || lookups[f.key]).endsWith("/members")
              ? "user_id"
              : "id"
          }
          labelKey={f.key === "resource_id" ? "database_name" : "display_name"}
          excludeControl={f.excludeControl}
          excludeInactive={
            f.excludeInactive ||
            [
              "user_id",
              "initial_admin_user_id",
              "initial_manager_user_id",
            ].includes(f.key)
          }
          excludePlatform={
            f.excludePlatform ||
            ["initial_admin_user_id", "initial_manager_user_id"].includes(f.key)
          }
          onChange={(value) => setValues({ ...values, [f.key]: value })}
        />
      ) : f.type === "json" ? (
        <textarea
          aria-label={f.label}
          required={f.required}
          rows={5}
          value={String(values[f.key] ?? "{}")}
          onChange={(e) => setValues({ ...values, [f.key]: e.target.value })}
        />
      ) : f.type === "select" ? (
        <Select
          aria-label={f.label}
          required={f.required}
          value={String(values[f.key] ?? "")}
          options={(f.options || []).map((o) => ({
            value: text(o.value),
            label: text(o.label),
          }))}
          onChange={(value) => setValues({ ...values, [f.key]: value })}
        />
      ) : f.type === "number" ? (
        <NumberInput
          aria-label={f.label}
          autoFocus={autoFocus}
          required={f.required}
          min={0}
          value={keyOf(values[f.key])}
          onChange={(value) => setValues({ ...values, [f.key]: value })}
        />
      ) : (
        <FormModalTextInput {...props} />
      )}
      {(f.hint || f.newPassword) && f.type !== "note" && (
        <small>
          {f.newPassword && `${policy.length_hint} `}
          {f.hint}
        </small>
      )}
    </Container>
  );
}
