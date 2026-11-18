import { useState } from "react";
import { Check } from "lucide-react";
import { RemoteSelect } from "./remote-select";
import { Select } from "./select";
import { NumberInput } from "./number-input";
import { Dialog, DialogFooter } from "./dialog";
import { Button } from "./button";
import { Alert } from "./alert";
import { toast } from "./toast-store";
import { Checkbox } from "./checkbox";
import { NewPasswordInput } from "./new-password-input";
import { generatePassword, usePasswordPolicy } from "../console-config";
import { useAction } from "../use-action";
import { keyOf, text, type Item } from "../api-types";
// A multi-value field's current list; unset means none chosen.
const listValue = (v: unknown): unknown[] => (Array.isArray(v) ? v : []);
export type Field = {
  key: string;
  label: string;
  type?: string;
  options?: Item[];
  required?: boolean;
  newPassword?: boolean;
  hint?: string;
  lookup?: string;
  value?: unknown;
  when?: (values: Item) => boolean;
  generate?: boolean;
  advanced?: boolean;
  excludeControl?: boolean;
  excludePlatform?: boolean;
  excludeInactive?: boolean;
  wide?: boolean;
};
export type Modal = {
  title: string;
  description?: string;
  fields: Field[];
  initial: Item;
  save: (v: Item) => Promise<unknown>;
  submit?: string;
  // The confirmation toast after a save; by default derived from the title
  // ("Create App" → "App created", "Edit team" → "Team saved").
  success?: string;
};

function savedMessage(title: string) {
  const [verb, ...rest] = title.split(" ");
  const subject = rest.join(" ");
  const done = { Create: "created", Add: "added", Edit: "saved" }[verb];
  return done && subject
    ? `${subject[0].toUpperCase()}${subject.slice(1)} ${done}.`
    : "Saved.";
}
export function FormModal({
  modal,
  close,
  done,
  lookups,
}: {
  modal: Modal;
  close: () => void;
  done: () => void;
  lookups: Record<string, string>;
}) {
  const policy = usePasswordPolicy();
  const [values, setValues] = useState<Item>(modal.initial);
  const [error, setError] = useState("");
  const [visible, setVisible] = useState<string[]>([]);
  const { busy, run } = useAction(modal, {
    onSuccess: () => {
      toast(modal.success || savedMessage(modal.title), { tone: "success" });
      done();
    },
    onError: setError,
  });
  function submit(e: React.FormEvent) {
    e.preventDefault();
    void run(() => modal.save(values));
  }
  return (
    <Dialog
      title={modal.title}
      description={modal.description}
      titleId="modal-title"
      className="form-modal"
      dismissable={!busy}
      onClose={close}
    >
      <form onSubmit={submit}>
        <div className="fields">
          {[false, true].map((advanced) => {
            const fields = modal.fields.filter(
              (f) => !!f.advanced === advanced && (!f.when || f.when(values)),
            );
            const content = fields.map((f, i) => {
              const Input = f.newPassword ? NewPasswordInput : "input";
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
                  key={f.key}
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
                        onChange={(checked) =>
                          setValues({ ...values, [f.key]: checked })
                        }
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
                                  : listValue(values[f.key]).filter(
                                      (x) => x !== o.value,
                                    ),
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
                      labelKey={
                        f.key === "resource_id"
                          ? "database_name"
                          : "display_name"
                      }
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
                        [
                          "initial_admin_user_id",
                          "initial_manager_user_id",
                        ].includes(f.key)
                      }
                      onChange={(value) =>
                        setValues({ ...values, [f.key]: value })
                      }
                    />
                  ) : f.type === "json" ? (
                    <textarea
                      aria-label={f.label}
                      required={f.required}
                      rows={5}
                      value={String(values[f.key] ?? "{}")}
                      onChange={(e) =>
                        setValues({ ...values, [f.key]: e.target.value })
                      }
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
                      onChange={(value) =>
                        setValues({ ...values, [f.key]: value })
                      }
                    />
                  ) : f.type === "number" ? (
                    <NumberInput
                      aria-label={f.label}
                      autoFocus={i === 0 && !advanced}
                      required={f.required}
                      min={0}
                      value={keyOf(values[f.key])}
                      onChange={(value) =>
                        setValues({ ...values, [f.key]: value })
                      }
                    />
                  ) : (
                    <span className="form-input-group">
                      <Input
                        aria-label={f.label}
                        autoFocus={i === 0 && !advanced}
                        required={f.required}
                        type={
                          f.type === "password" && visible.includes(f.key)
                            ? "text"
                            : f.type || "text"
                        }
                        autoComplete={
                          f.type === "password" ? "new-password" : "off"
                        }
                        value={String(values[f.key] ?? "")}
                        onChange={(e) => {
                          const v: Item = {
                            ...values,
                            [f.key]: e.target.value,
                          };
                          for (const key of ["slug", "code"]) {
                            const clean = (name: string) =>
                              name
                                .toLowerCase()
                                .replace(/[^a-z0-9]+/g, "-")
                                .replace(/^[^a-z]+|[-]+$/g, "")
                                .slice(0, 63) || "item";
                            if (
                              f.key === "display_name" &&
                              modal.fields.some((field) => field.key === key) &&
                              (!values[key] ||
                                values[key] ===
                                  clean(String(values.display_name || "")))
                            )
                              v[key] = clean(e.target.value);
                          }
                          setValues(v);
                        }}
                      />
                      {f.generate && (
                        <span className="row-actions">
                          <button
                            type="button"
                            className="text-button"
                            onClick={() => {
                              setValues({
                                ...values,
                                [f.key]: generatePassword(policy),
                              });
                              setVisible((old) => [...old, f.key]);
                            }}
                          >
                            Generate password
                          </button>
                          <button
                            type="button"
                            className="text-button"
                            onClick={() =>
                              setVisible((old) =>
                                old.includes(f.key)
                                  ? old.filter((k) => k !== f.key)
                                  : [...old, f.key],
                              )
                            }
                          >
                            {visible.includes(f.key) ? "Hide" : "Show"}
                          </button>
                        </span>
                      )}
                    </span>
                  )}
                  {(f.hint || f.newPassword) && f.type !== "note" && (
                    <small>
                      {f.newPassword && `${policy.length_hint} `}
                      {f.hint}
                    </small>
                  )}
                </Container>
              );
            });
            return advanced ? (
              fields.length > 0 && (
                <details className="advanced-fields wide" key="advanced">
                  <summary>Advanced</summary>
                  <div className="fields">{content}</div>
                </details>
              )
            ) : (
              <span className="basic-fields" key="basic">
                {content}
              </span>
            );
          })}
        </div>
        {error && <Alert>{error}</Alert>}
        <DialogFooter>
          <Button disabled={busy} onClick={close}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            loading={busy}
            icon={<Check size={16} aria-hidden />}
          >
            {busy ? "Saving…" : modal.submit || "Save changes"}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}
