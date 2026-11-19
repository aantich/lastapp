// "Tuesday, 17 November": the weekday, then the date in the reader's locale.
export function today() {
  const now = new Date();
  return `${now.toLocaleDateString(undefined, { weekday: "long" })}, ${now.toLocaleDateString(undefined, { day: "numeric", month: "long" })}`;
}

export function greeting() {
  const hour = new Date().getHours();
  return hour < 12
    ? "Good morning"
    : hour < 18
      ? "Good afternoon"
      : "Good evening";
}
