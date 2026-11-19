// UI code rules for stylesheets, run by `npm run lint` (and so `make lint`).
// ESLint covers TypeScript; this covers every .css file under src/:
//   - at most 200 lines per file, not counting blank and comment lines (the
//     same limit and counting as ESLint's max-lines for TypeScript);
//   - outside src/theme/ (the Signal tokens), no color literal and no literal
//     length or duration in a declaration, and z-index only from a token.
// Media query breakpoints are exempt: custom properties cannot be used there.
import { readdirSync, readFileSync } from "node:fs";
import { join, relative, sep } from "node:path";

const root = new URL("..", import.meta.url).pathname;
const src = join(root, "src");
const tokens = join(src, "theme") + sep;
const maxLines = 200;

// CSS named colors (CSS Color 4) other than transparent and currentColor.
const named = `aliceblue antiquewhite aqua aquamarine azure beige bisque black
blanchedalmond blue blueviolet brown burlywood cadetblue chartreuse chocolate
coral cornflowerblue cornsilk crimson cyan darkblue darkcyan darkgoldenrod
darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange
darkorchid darkred darksalmon darkseagreen darkslateblue darkslategray
darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey
dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite gold
goldenrod gray green greenyellow grey honeydew hotpink indianred indigo ivory
khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral
lightcyan lightgoldenrodyellow lightgray lightgreen lightgrey lightpink
lightsalmon lightseagreen lightskyblue lightslategray lightslategrey
lightsteelblue lightyellow lime limegreen linen magenta maroon
mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen
mediumslateblue mediumspringgreen mediumturquoise mediumvioletred
midnightblue mintcream mistyrose moccasin navajowhite navy oldlace olive
olivedrab orange orangered orchid palegoldenrod palegreen paleturquoise
palevioletred papayawhip peachpuff peru pink plum powderblue purple
rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown seagreen
seashell sienna silver skyblue slateblue slategray slategrey snow springgreen
steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke
yellow yellowgreen`.split(/\s+/);

const checks = [
  [/#[\da-f]{3,8}\b/i, "color literal (use a --ui-* color token)"],
  [
    /\b(rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\(/i,
    "color function (use a --ui-* color token)",
  ],
  [
    new RegExp(`(^|[\\s,(])(${named.join("|")})(?=$|[\\s,);])`, "i"),
    "named color (use a --ui-* color token)",
  ],
  [
    /(^|[^\w.#-])-?(\d*\.)?\d*[1-9]\d*(px|rem|em|ex|ch|pt|pc|cm|mm|in|q|ms|s)\b/i,
    "literal length or duration (use a --ui-* scale token)",
  ],
];

function files(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory()
      ? files(join(dir, e.name))
      : e.name.endsWith(".css")
        ? [join(dir, e.name)]
        : [],
  );
}

// Blank out comments, keeping line breaks so positions keep their line.
const uncomment = (css) =>
  css.replace(/\/\*[\s\S]*?\*\//g, (c) => c.replace(/[^\n]/g, " "));
const lineOf = (text, index) => text.slice(0, index).split("\n").length;

const problems = [];
for (const file of files(src)) {
  const name = relative(root, file);
  const css = uncomment(readFileSync(file, "utf8"));
  const lines = css.split("\n").filter((l) => l.trim()).length;
  if (lines > maxLines)
    problems.push(`${name}: ${lines} lines (maximum ${maxLines})`);
  if (file.startsWith(tokens)) continue;
  // A declaration ends at ";" or "}"; selectors and at-rule preludes end at "{".
  for (const m of css.matchAll(/([-\w]+)\s*:\s*([^;{}]+)(?=[;}])/g)) {
    const [, property, value] = m;
    const at = `${name}:${lineOf(css, m.index)}`;
    if (property === "z-index" && !/^(var\(|auto$|0$)/.test(value.trim()))
      problems.push(`${at}: z-index ${value.trim()} (use a --ui-z-* token)`);
    for (const [pattern, message] of checks)
      if (pattern.test(value))
        problems.push(`${at}: ${property}: ${value.trim()}: ${message}`);
  }
}

if (problems.length) {
  console.error(problems.join("\n"));
  console.error(`\n${problems.length} stylesheet rule violation(s)`);
  process.exit(1);
}
