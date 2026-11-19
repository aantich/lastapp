import type { Dispatch, SetStateAction } from "react";
import { NewPasswordInput } from "./new-password-input";
import { generatePassword } from "../console-config";
import type { Item, PasswordPolicy } from "../api-types";
import type { Field } from "./form-modal-types";

export type FieldInputProps = {
  f: Field;
  fields: Field[];
  autoFocus: boolean;
  values: Item;
  setValues: (values: Item) => void;
  visible: string[];
  setVisible: Dispatch<SetStateAction<string[]>>;
  policy: PasswordPolicy;
};

// A free-text field: the plain/password input, the slug/code derivation from
// the display name and the password generate/show actions.
export function FormModalTextInput({
  f,
  fields,
  autoFocus,
  values,
  setValues,
  visible,
  setVisible,
  policy,
}: FieldInputProps) {
  const Input = f.newPassword ? NewPasswordInput : "input";
  return (
    <span className="form-input-group">
      <Input
        aria-label={f.label}
        autoFocus={autoFocus}
        required={f.required}
        type={
          f.type === "password" && visible.includes(f.key)
            ? "text"
            : f.type || "text"
        }
        autoComplete={f.type === "password" ? "new-password" : "off"}
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
              fields.some((field) => field.key === key) &&
              (!values[key] ||
                values[key] === clean(String(values.display_name || "")))
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
  );
}
