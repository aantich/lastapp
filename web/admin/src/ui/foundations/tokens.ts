import themes from "../../themes.css?raw";
import base from "../../style.css?raw";

// Foundations stories read the token declarations from the stylesheets
// themselves, so the catalogue cannot drift from themes.css.
function block(css: string, selector: string) {
  const start = css.indexOf(selector + " {");
  if (start < 0) return {};
  const body = css.slice(start, css.indexOf("}", start));
  const tokens: Record<string, string> = {};
  for (const [, name, value] of body.matchAll(/(--ui-[\w-]+):\s*([^;]+);/g))
    tokens[name] = value.replace(/\s+/g, " ").trim();
  return tokens;
}

export const light = block(themes, ":root");
export const dark = { ...light, ...block(themes, ':root[data-mode="dark"]') };
export const sidebarScope = block(themes, ".sidebar");
// Layout tokens declared with the base styles (spacing, control height).
export const layout = block(base, ":root");

export const isColor = (value: string) => /^(#|rgb)/.test(value);

function channels(value: string): [number, number, number] | null {
  const hex = value.match(/^#([\da-f]{6})$/i);
  if (hex)
    return [0, 2, 4].map((i) => parseInt(hex[1].slice(i, i + 2), 16)) as [
      number,
      number,
      number,
    ];
  return null; // Translucent rgb() overlays have no fixed contrast.
}

function luminance(value: string) {
  const c = channels(value);
  if (!c) return null;
  const [r, g, b] = c.map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

// WCAG 2.x contrast ratio, or null when either color is not opaque hex.
export function contrast(a: string, b: string) {
  const x = luminance(a),
    y = luminance(b);
  if (x === null || y === null) return null;
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
}

// CIE L* (0–100): status colors must differ in lightness, not hue alone.
export function lightness(value: string) {
  const y = luminance(value);
  if (y === null) return null;
  return y > 0.008856 ? 116 * Math.cbrt(y) - 16 : 903.3 * y;
}
