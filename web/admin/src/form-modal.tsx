import { useEffect, useRef, useState } from "react";
import { Check, LoaderCircle, X } from "lucide-react";
import { RemoteSelect } from "./remote-select";
import { Select } from "./select";
import { NumberInput } from "./number-input";
import { useDialogFocus } from "./dialog";
import {
  NewPasswordInput,
  usePasswordPolicy,
  generatePassword,
} from "./new-password-input";
import { errorMessage } from "./use-action";
import type { Item } from "./api-types";
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
};
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
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [visible, setVisible] = useState<string[]>([]);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const dialogRef = useDialogFocus(() => {
    if (!busy) close();
  });
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await modal.save(values);
      if (mounted.current) done();
    } catch (e) {
      if (mounted.current) {
        setError(errorMessage(e));
        setBusy(false);
      }
    }
  }
  return (
    <div
      className="overlay"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !busy) close();
      }}
    >
      <section
        ref={dialogRef}
        className="modal form-modal"
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
      >
        <header>
          <div>
            <h2 id="modal-title">{modal.title}</h2>
            {modal.description && <p>{modal.description}</p>}
          </div>
          <button
            className="icon"
            aria-label="Close dialog"
            disabled={busy}
            onClick={close}
          >
            <X size={19} />
          </button>
        </header>
        <form onSubmit={submit}>
          <div className="fields">
            {[false, true].map((advanced) => {
              const fields = modal.fields.filter(
                (f) => !!f.advanced === advanced && (!f.when || f.when(values)),
              );
              const content = fields.map((f, i) => {
                const Input = f.newPassword ? NewPasswordInput : "input";
                const Container =
                  f.type === "permissions" || f.type === "note"
                    ? "div"
                    : "label";
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
                        <input
                          type="checkbox"
                          checked={!!values[f.key]}
                          onChange={(e) =>
                            setValues({ ...values, [f.key]: e.target.checked })
                          }
                        />
                        {f.label}
                      </span>
                    ) : f.type === "permissions" ? (
                      <div className="permission-list">
                        {f.options?.map((o) => (
                          <label key={o.value}>
                            <input
                              type="checkbox"
                              checked={(values[f.key] || []).includes(o.value)}
                              onChange={(e) =>
                                setValues({
                                  ...values,
                                  [f.key]: e.target.checked
                                    ? [...(values[f.key] || []), o.value]
                                    : (values[f.key] || []).filter(
                                        (x: string) => x !== o.value,
                                      ),
                                })
                              }
                            />
                            <span>{o.label}</span>
                          </label>
                        ))}
                      </div>
                    ) : f.type === "select" && (f.lookup || lookups[f.key]) ? (
                      <RemoteSelect
                        label={f.label}
                        required={f.required}
                        value={values[f.key] ?? ""}
                        initial={(f.options || []).map((o) => ({
                          value: o.value,
                          label: o.label,
                        }))}
                        path={f.lookup || lookups[f.key]}
                        valueKey={
                          (f.lookup || lookups[f.key]).endsWith(
                            "memberships",
                          ) || (f.lookup || lookups[f.key]).endsWith("/members")
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
                        value={values[f.key] ?? "{}"}
                        onChange={(e) =>
                          setValues({ ...values, [f.key]: e.target.value })
                        }
                      />
                    ) : f.type === "select" ? (
                      <Select
                        aria-label={f.label}
                        required={f.required}
                        value={values[f.key] ?? ""}
                        options={(f.options || []).map((o) => ({
                          value: o.value,
                          label: o.label,
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
                        value={values[f.key]}
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
                          value={values[f.key] ?? ""}
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
                                modal.fields.some(
                                  (field) => field.key === key,
                                ) &&
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
          {error && (
            <div className="error" role="alert">
              {error}
            </div>
          )}
          <footer>
            <button
              type="button"
              className="secondary"
              disabled={busy}
              onClick={close}
            >
              Cancel
            </button>
            <button className="primary" disabled={busy}>
              {busy ? (
                <LoaderCircle className="spin" size={16} />
              ) : (
                <Check size={16} />
              )}{" "}
              {busy ? "Saving…" : modal.submit || "Save changes"}
            </button>
          </footer>
        </form>
      </section>
    </div>
  );
}
