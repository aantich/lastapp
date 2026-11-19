import { api } from "../api";
import {
  num,
  type Item,
  type Page,
  type Release,
  type ReleaseItem,
  type ReleaseManifest,
} from "../api-types";
import type { Modal } from "../ui";

// The "Create release" form for one uploaded build: it pins every active
// micro-app version, prefilled from the latest release.
export async function releaseModal(
  tenant: string,
  app: Item,
  buildID: string,
  releases: ReleaseItem[],
  maxMicroapps: number,
  created: (release: Release) => void,
): Promise<Modal> {
  const [latest, inventory] = await Promise.all([
    api<Item>(`/v1/tenants/${tenant}/apps/${app.id}`),
    api<Page>(`/v1/tenants/${tenant}/microapps?app_id=${app.id}&limit=200`),
  ]);
  const activeMicros = inventory.items.filter((m) => m.state === "active");
  if (inventory.pagination?.has_more || activeMicros.length > maxMicroapps)
    throw new Error("This App exceeds the release micro-app limit.");
  let previous: ReleaseManifest = {};
  if (releases[0])
    previous =
      (await api<Release>("/v1/releases/" + releases[0].id)).manifest_json ||
      {};
  return {
    title: "Create release",
    description:
      "Pin every micro-app version. The release also captures the published Space entities, rules and ACLs for review.",
    success: "Release created",
    initial: {
      display_name: "",
      dependencies: JSON.stringify(previous.dependencies || {}, null, 2),
      ...Object.fromEntries(
        activeMicros.map((m) => [
          "micro_" + m.id,
          previous.microapps?.[String(m.slug)] || "1.0.0",
        ]),
      ),
    },
    fields: [
      { key: "display_name", label: "Release name", required: true },
      ...activeMicros.map((m) => ({
        key: "micro_" + m.id,
        label: m.display_name + " version",
        required: true,
        hint: "Keep the previous version for an unchanged micro-app.",
      })),
      {
        key: "dependencies",
        label: "Shared package versions",
        type: "json",
        wide: true,
        advanced: true,
        hint: "Exact versions from your lockfile, as a JSON object. Your agent supplies these through MCP.",
      },
    ],
    submit: "Create release",
    save: async (v) => {
      const rel = await api<Release>(
        `/v1/apps/${app.id}/releases`,
        "POST",
        {
          display_name: v.display_name,
          build_id: buildID,
          manifest: {
            microapps: Object.fromEntries(
              activeMicros.map((m) => [m.slug, v["micro_" + m.id]]),
            ),
            dependencies: JSON.parse(String(v.dependencies || "{}")),
          },
        },
        num(latest.revision),
      );
      created(rel);
    },
  };
}
