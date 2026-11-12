// Apply saved appearance before CSS loads to avoid a light flash in dark mode.
try {
  const saved = JSON.parse(
    localStorage.getItem("lastapp-appearance") || "null",
  );
  const root = document.documentElement;
  root.dataset.theme = ["neo", "studio", "evergreen"].includes(saved?.theme)
    ? saved.theme
    : "neo";
  root.dataset.mode =
    saved?.mode === "dark" ||
    (saved?.mode === "system" &&
      matchMedia("(prefers-color-scheme: dark)").matches)
      ? "dark"
      : "light";
} catch {}
