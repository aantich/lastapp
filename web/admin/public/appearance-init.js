// Apply the saved color mode before CSS loads to avoid a light flash in dark
// mode. Mirrors readAppearance/applyMode in src/appearance.ts; an older saved
// `theme` preset is ignored because Signal is the only theme.
try {
  const saved = JSON.parse(
    localStorage.getItem("lastapp-appearance") || "null",
  );
  const dark =
    saved?.mode === "dark" ||
    (saved?.mode === "system" &&
      matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.mode = dark ? "dark" : "light";
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", dark ? "#141518" : "#f6f5f2");
} catch {}
