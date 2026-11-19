import { text } from "../api-types";

export const when = (at: unknown) => new Date(text(at)).toLocaleString();

// "09:41" today, otherwise the date and time, in the reader's locale.
export function time(at: string) {
  const d = new Date(at);
  return d.toDateString() === new Date().toDateString()
    ? d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })
    : d.toLocaleString(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      });
}
