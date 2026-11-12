import { useEffect, useRef, useState } from "react";
import { Check, LoaderCircle, X } from "lucide-react";
import { RemoteSelect } from "./remote-select";
import { useDialogFocus } from "./appearance";
type Item = Record<string, any>;
export type Field = {
  key: string;
  label: string;
  type?: string;
  options?: Item[];
  required?: boolean;
  hint?: string;
  lookup?: string;
  value?: any;
  when?: (values: Item) => boolean;
  generate?: boolean;
};
export type Modal = {
  title: string;
  description?: string;
  fields: Field[];
  initial: Item;
  save: (v: Item) => Promise<any>;
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
        setError((e as Error).message);
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
        className="modal"
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
            {modal.fields
              .filter((f) => !f.when || f.when(values))
              .map((f, i) => (
                <label
                  className={
                    f.type === "permissions" || f.type === "note" ? "wide" : ""
                  }
                  key={f.key}
                >
                  {f.type !== "checkbox" && (
                    <span>
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
                      excludeInactive={[
                        "user_id",
                        "initial_admin_user_id",
                        "initial_manager_user_id",
                      ].includes(f.key)}
                      excludePlatform={[
                        "initial_admin_user_id",
                        "initial_manager_user_id",
                      ].includes(f.key)}
                      onChange={(value) =>
                        setValues({ ...values, [f.key]: value })
                      }
                    />
                  ) : f.type === "select" ? (
                    <select
                      aria-label={f.label}
                      required={f.required}
                      value={values[f.key] ?? ""}
                      onChange={(e) =>
                        setValues({ ...values, [f.key]: e.target.value })
                      }
                    >
                      <option value="">Choose…</option>
                      {f.options?.map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <span className="form-input-group">
                      <input
                        aria-label={f.label}
                        autoFocus={i === 0}
                        required={f.required}
                        min={f.type === "number" ? 0 : undefined}
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
                          if (
                            f.key === "display_name" &&
                            (!values.slug ||
                              values.slug ===
                                String(values.display_name || "")
                                  .toLowerCase()
                                  .replace(/[^a-z0-9]+/g, "-")
                                  .slice(0, 63)) &&
                            modal.fields.some((f) => f.key === "slug")
                          )
                            v.slug = e.target.value
                              .toLowerCase()
                              .replace(/[^a-z0-9]+/g, "-")
                              .slice(0, 63);
                          setValues(v);
                        }}
                      />
                      {f.generate && (
                        <span className="row-actions">
                          <button
                            type="button"
                            className="text-button"
                            onClick={() => {
                              const bytes = crypto.getRandomValues(
                                new Uint8Array(20),
                              );
                              setValues({
                                ...values,
                                [f.key]: Array.from(bytes, (b) =>
                                  b.toString(16).padStart(2, "0"),
                                ).join(""),
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
                  {f.hint && f.type !== "note" && <small>{f.hint}</small>}
                </label>
              ))}
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
