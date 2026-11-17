// Small helpers shared by kit components.

// Joins class names, skipping false/empty values.
export function cx(...names: (string | false | null | undefined)[]) {
  return names.filter(Boolean).join(" ");
}

// Up to two initials from a display name ("Lena Novak" → "LN").
export function initials(name: string) {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (!words.length) return "?";
  const letters =
    words.length === 1
      ? Array.from(words[0]).slice(0, 2)
      : [Array.from(words[0])[0], Array.from(words.at(-1)!)[0]];
  return letters.join("").toUpperCase();
}
