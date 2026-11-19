import { useEffect, useRef, useState } from "react";
import { Textarea } from "../ui";

export function JSONInput({
  value,
  onChange,
  label,
  hideLabel,
  allowEmpty = false,
}: {
  value: unknown;
  onChange: (v: unknown) => void;
  label: string;
  hideLabel?: boolean;
  allowEmpty?: boolean;
}) {
  const serialize = (v: unknown) =>
    v === undefined ? "" : JSON.stringify(v, null, 2);
  const [text, setText] = useState(() => serialize(value));
  const last = useRef<string | undefined>(JSON.stringify(value));
  useEffect(() => {
    const next = JSON.stringify(value);
    if (next !== last.current) {
      last.current = next;
      setText(serialize(value));
    }
  }, [value]);
  return (
    <Textarea
      label={label}
      hideLabel={hideLabel}
      rows={5}
      value={text}
      placeholder={allowEmpty ? "Omit, or enter a JSON object or list" : ""}
      onChange={(e) => {
        const raw = e.target.value;
        setText(raw);
        if (!raw.trim() && allowEmpty) {
          last.current = undefined;
          e.currentTarget.setCustomValidity("");
          onChange(undefined);
          return;
        }
        try {
          const parsed: unknown = JSON.parse(raw);
          if (parsed === null || typeof parsed !== "object") throw new Error();
          last.current = JSON.stringify(parsed);
          e.currentTarget.setCustomValidity("");
          onChange(parsed);
        } catch {
          e.currentTarget.setCustomValidity(
            "Enter a valid JSON object or list.",
          );
        }
      }}
    />
  );
}
