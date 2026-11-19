// Apply the saved color mode before CSS loads to avoid a light flash in dark
// mode. Mirrors readAppearance/applyMode in src/appearance.ts, which also sets
// the browser chrome color from the --ui-canvas token once the styles load; an
// older saved `theme` preset is ignored because Signal is the only theme.
try {
  const saved = JSON.parse(
    localStorage.getItem("lastapp-appearance") || "null",
  );
  const dark =
    saved?.mode === "dark" ||
    (saved?.mode === "system" &&
      matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.mode = dark ? "dark" : "light";
} catch {}
