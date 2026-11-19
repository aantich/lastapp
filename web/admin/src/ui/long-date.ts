// A day spelled out in the viewer's locale ("Monday, January 1, 2024").
export const longDate = (d: Date) =>
  d.toLocaleDateString(undefined, {
    weekday: "long",
    year: "numeric",
    month: "long",
    day: "numeric",
  });
