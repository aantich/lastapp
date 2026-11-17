import { useLayoutEffect, useRef, useState, type ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { light } from "./tokens";

// Each sample is the real element styled by style.css; the caption is
// measured from the rendered element, not restated here.
function Sample({ name, children }: { name: string; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  const [spec, setSpec] = useState("");
  useLayoutEffect(() => {
    const el = ref.current?.firstElementChild;
    if (!el) return;
    const s = getComputedStyle(el);
    const family = s.fontFamily.split(",")[0].replace(/"/g, "");
    setSpec(`${family} · ${s.fontSize} / ${s.lineHeight} · ${s.fontWeight}`);
  }, []);
  return (
    <div className="grid gap-1 border-b border-line py-4 last:border-0">
      <small className="m-0">
        {name} — {spec}
      </small>
      <div ref={ref}>{children}</div>
    </div>
  );
}

function Typography() {
  return (
    <div className="grid gap-6">
      <section className="panel p-5">
        <h2 className="mb-2">Families</h2>
        <ul className="grid gap-1 text-muted">
          {["--ui-font", "--ui-font-heading", "--ui-font-mono"].map((name) => (
            <li key={name}>
              <code>{name}</code>: {light[name]}
            </li>
          ))}
        </ul>
      </section>
      <section className="panel px-5">
        <Sample name="h1 · page title">
          <h1>Northstar CRM</h1>
        </Sample>
        <Sample name="h2 · section and dialog title">
          <h2>Deal pipeline</h2>
        </Sample>
        <Sample name="h3 · card title">
          <h3>Renewal — Acme Logistics</h3>
        </Sample>
        <Sample name="Body">
          <p>
            The CRM Space owns the Deal entity. Apps reuse its records and may
            narrow who can edit the stage and amount.
          </p>
        </Sample>
        <Sample name="Muted helper (small)">
          <small>Last published 12 November 2026 by Ava Chen.</small>
        </Sample>
        <Sample name="Identifier (code)">
          <code>spc_0001 · deal.stage · app_northstar_crm</code>
        </Sample>
      </section>
    </div>
  );
}

const meta = { title: "Foundations/Typography" } satisfies Meta;
export default meta;
export const Scale: StoryObj<typeof meta> = { render: () => <Typography /> };
