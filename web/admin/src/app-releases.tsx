import { useEffect, useRef, useState } from "react";
import { Download, Upload, RefreshCw, ExternalLink } from "lucide-react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { api, type Failure } from "./api";
import { apiKey, queryClient, refreshQueries, useApiQuery } from "./query";
import { useConsoleConfig } from "./console-config";
import { FormModal, type Modal } from "./form-modal";
import { status } from "./console-forms";
import type { Item, Page } from "./api-types";
import "./development-workspaces.css";
import { errorMessage } from "./use-action";

async function encode(file: File) {
  const bytes = new Uint8Array(await file.arrayBuffer());
  let text = "";
  for (let i = 0; i < bytes.length; i += 8192)
    text += String.fromCharCode(...bytes.subarray(i, i + 8192));
  return btoa(text);
}
export function AppReleases({
  app,
  tenant,
  refresh,
  openWorkspaces,
}: {
  app: Item;
  tenant: string;
  refresh: () => void;
  openWorkspaces: () => void;
}) {
  const [selected, setSelected] = useState<Item>(),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [modal, setModal] = useState<Modal | null>(null);
  const { config } = useConsoleConfig();
  const folder = useRef<HTMLInputElement>(null),
    live = useRef(true),
    development = !!app.workspace_id;
  useEffect(() => {
    live.current = true;
    return () => {
      live.current = false;
    };
  }, []);
  // Release history pages accumulate behind "Load more".
  const releasesPath = `/v1/apps/${app.id}/releases`;
  const releasePages = useInfiniteQuery({
    queryKey: [...apiKey(releasesPath), "pages"],
    queryFn: ({ pageParam, signal }) =>
      api<Page>(
        `${releasesPath}?page=${pageParam}`,
        "GET",
        undefined,
        undefined,
        signal,
      ),
    initialPageParam: 1,
    getNextPageParam: (last, _, page) =>
      last.pagination?.has_more ? page + 1 : undefined,
  });
  const releases = (releasePages.data?.pages || []).reduce<Item[]>(
      (all, p) => [
        ...all.filter((r) => !p.items.some((n) => n.id === r.id)),
        ...p.items,
      ],
      [],
    ),
    hasMore = !!releasePages.hasNextPage,
    listError = releasePages.error?.message || "";
  const builds =
    useApiQuery<Page>(development ? `/v1/apps/${app.id}/builds` : null).data
      ?.items || [];
  // Production deployment status: poll while it is in progress, backing off
  // from 1.5 s to 10 s after failures, and stop at a final state or when
  // access is gone. refetchInterval never overlaps requests.
  const deploymentPath = `/v1/apps/${app.id}/deployment`,
    failures = useRef({ at: 0, count: 0 });
  const deploymentQuery = useApiQuery<{ deployment?: Item | null }>(
    deploymentPath,
    {
      refetchInterval: (q) => {
        const { status, error, errorUpdatedAt, data } = q.state;
        if (status === "error") {
          if (failures.current.at !== errorUpdatedAt)
            failures.current = {
              at: errorUpdatedAt,
              count: failures.current.count + 1,
            };
          if ([401, 403, 404].includes((error as Failure).status)) return false;
          return Math.min(1500 * 2 ** failures.current.count, 10000);
        }
        failures.current = { at: 0, count: 0 };
        return data?.deployment &&
          ["queued", "preparing", "publishing"].includes(data.deployment.state)
          ? 1500
          : false;
      },
    },
  );
  const deployment = deploymentQuery.data?.deployment || undefined,
    deploymentError = deploymentQuery.error
      ? `Could not refresh deployment status: ${errorMessage(deploymentQuery.error)}`
      : "";
  // When this deployment finishes, the App itself changed: refresh it.
  const previousDeployment = useRef<Item | undefined>(undefined);
  useEffect(() => {
    const previous = previousDeployment.current;
    previousDeployment.current = deployment;
    if (
      deployment?.state === "active" &&
      previous &&
      previous.id === deployment.id &&
      previous.state !== "active"
    )
      refresh();
  }, [deployment, refresh]);
  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (e) {
      if (live.current) setError(errorMessage(e));
    } finally {
      if (live.current) setBusy(false);
    }
  }
  async function upload(files: FileList | null) {
    if (!files?.length) return;
    await run(async () => {
      const ticket = await api<Item>(
          `/v1/apps/${app.id}/upload-tickets`,
          "POST",
          {},
        ),
        list = Array.from(files);
      if (
        list.length > ticket.limits.max_files ||
        list.some((f) => f.size > ticket.limits.max_file_bytes) ||
        list.reduce((n, f) => n + f.size, 0) > ticket.limits.max_bytes
      )
        throw new Error("The build exceeds the upload limits.");
      const content = await Promise.all(
        list.map(async (f) => ({
          path: f.webkitRelativePath.split("/").slice(1).join("/"),
          content_base64: await encode(f),
        })),
      );
      const response = await fetch(ticket.upload_url, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${ticket.upload_token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ entrypoint: "index.html", files: content }),
      });
      const result = await response.json();
      if (!response.ok)
        throw new Error(result.error?.message || "Upload failed");
      void refreshQueries();
      await capture(result.id);
    });
    if (folder.current) folder.current.value = "";
  }
  async function capture(buildID: string) {
    const [latest, inventory] = await Promise.all([
      api<Item>(`/v1/tenants/${tenant}/apps/${app.id}`),
      api<Page>(`/v1/tenants/${tenant}/microapps?app_id=${app.id}&limit=200`),
    ]);
    const activeMicros = inventory.items.filter((m) => m.state === "active");
    if (
      inventory.pagination?.has_more ||
      activeMicros.length > config.development_policy.max_release_microapps
    )
      throw new Error("This App exceeds the release micro-app limit.");
    let previous: Item = {};
    if (releases[0])
      previous =
        (await api<Item>("/v1/releases/" + releases[0].id)).manifest_json || {};
    setModal({
      title: "Create release",
      description:
        "Pin every micro-app version. The release also captures the published Space entities, rules and ACLs for review.",
      initial: {
        display_name: "",
        dependencies: JSON.stringify(previous.dependencies || {}, null, 2),
        ...Object.fromEntries(
          activeMicros.map((m) => [
            "micro_" + m.id,
            previous.microapps?.[m.slug] || "1.0.0",
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
        const rel = await api<Item>(
          `/v1/apps/${app.id}/releases`,
          "POST",
          {
            display_name: v.display_name,
            build_id: buildID,
            manifest: {
              microapps: Object.fromEntries(
                activeMicros.map((m) => [m.slug, v["micro_" + m.id]]),
              ),
              dependencies: JSON.parse(v.dependencies || "{}"),
            },
          },
          latest.revision,
        );
        setSelected(rel);
      },
    });
  }
  async function preview() {
    if (!selected) return;
    const current = await api<Item>(`/v1/tenants/${tenant}/apps/${app.id}`),
      result = await api<{ url: string }>(
        `/v1/releases/${selected.id}/preview`,
        "POST",
        {},
        current.revision,
      );
    refresh();
    window.open(result.url, "_blank", "noopener,noreferrer");
  }
  function deploy() {
    if (!selected) return;
    setModal({
      title: "Deploy " + selected.display_name,
      description:
        "Applies the captured shared Space backend configuration, then activates this App build. Other Apps reuse those Space entities and permissions. Database changes are not automatically rolled back if a step fails.",
      initial: {},
      fields: [],
      submit: "Deploy to production",
      save: async () => {
        let revision = 1;
        try {
          const target = await api<Item>(
            `/v1/tenants/${tenant}/apps/${selected.production_app_id}`,
          );
          revision = target.revision;
        } catch (e) {
          if ((e as { status?: number }).status !== 404) throw e;
        }
        const result = await api<Item>(
          `/v1/releases/${selected.id}/deploy`,
          "POST",
          {},
          revision,
        );
        // Show the new deployment at once; its state restarts the polling.
        previousDeployment.current = result;
        queryClient.setQueryData(apiKey(deploymentPath), {
          deployment: result,
        });
      },
    });
  }
  return (
    <section className="development-page">
      <div className="development-heading">
        <div>
          <h2>Releases</h2>
          <p>
            {development
              ? "Build locally, test this workspace, then deploy a reviewed release."
              : "Production runs a selected immutable release. Develop changes in an isolated workspace."}
          </p>
        </div>
        <div className="development-actions">
          <button
            aria-label="Refresh releases"
            disabled={busy}
            onClick={() => void refreshQueries()}
          >
            <RefreshCw size={16} />
          </button>
          {development ? (
            <>
              <button
                disabled={busy}
                onClick={() => {
                  window.location.href = `/v1/apps/${app.id}/starter.zip`;
                }}
              >
                <Download size={16} />
                Get starter
              </button>
              <button
                className="primary"
                disabled={busy}
                onClick={() => {
                  folder.current?.setAttribute("webkitdirectory", "");
                  folder.current?.click();
                }}
              >
                <Upload size={16} />
                {busy ? "Working…" : "Upload build folder"}
              </button>
              <input
                ref={folder}
                type="file"
                multiple
                hidden
                onChange={(e) => upload(e.target.files)}
              />
            </>
          ) : (
            <button className="primary" onClick={openWorkspaces}>
              Open development workspaces
            </button>
          )}
        </div>
      </div>
      {(error || listError || deploymentError) && (
        <div className="error" role="alert">
          {error || listError || deploymentError}
        </div>
      )}
      {deployment && (
        <div className="panel deployment-status" role="status">
          <strong>Production deployment</strong>
          {status(deployment.state)}
          {deployment.error_message && (
            <p className="error">{deployment.error_message}</p>
          )}
          {deployment.state === "active" && (
            <a href={deployment.url} target="_blank" rel="noreferrer">
              Open production App <ExternalLink size={14} />
            </a>
          )}
        </div>
      )}
      <div className="release-layout">
        <div className="panel release-list">
          <h3>Release history</h3>
          {!releases.length && <p className="muted">No releases yet.</p>}
          {releases.map((r) => (
            <button
              key={r.id}
              className={
                "release-item " + (selected?.id === r.id ? "selected" : "")
              }
              onClick={() =>
                run(async () =>
                  setSelected(await api<Item>("/v1/releases/" + r.id)),
                )
              }
            >
              <strong>{r.display_name}</strong>
              <small>{new Date(r.created_at).toLocaleString()}</small>
            </button>
          ))}
          {hasMore && (
            <button onClick={() => void releasePages.fetchNextPage()}>
              Load more
            </button>
          )}
        </div>
        {selected ? (
          <article className="panel release-review">
            <div className="development-heading">
              <div>
                <h3>{selected.display_name}</h3>
                <p>Immutable build · {selected.build_id.slice(0, 8)}</p>
              </div>
              <div className="development-actions">
                {development && (
                  <button disabled={busy} onClick={() => run(preview)}>
                    Open preview <ExternalLink size={14} />
                  </button>
                )}
                <button className="primary" disabled={busy} onClick={deploy}>
                  Deploy to production
                </button>
              </div>
            </div>
            <h4>Captured Space backend</h4>
            <p>
              {selected.snapshot_json.entities.length} entities ·{" "}
              {selected.snapshot_json.roles.length} business roles ·{" "}
              {selected.snapshot_json.apps.length} Apps
            </p>
            <div className="release-entities">
              {selected.snapshot_json.entities.map((e: Item) => (
                <details key={e.id}>
                  <summary>
                    {e.definition.label}{" "}
                    <span className="muted">
                      {e.definition.fields.length} fields ·{" "}
                      {e.definition.permissions.length} permissions
                    </span>
                  </summary>
                  <div className="release-field-list">
                    {e.definition.fields.map((f: Item) => (
                      <div key={f.key}>
                        <strong>{f.label}</strong>
                        <span>
                          {f.type}
                          {f.required ? " · Required" : ""}
                        </span>
                      </div>
                    ))}
                  </div>
                </details>
              ))}
            </div>
            <details>
              <summary>Manifest and full configuration</summary>
              <pre>
                {JSON.stringify(
                  {
                    manifest: selected.manifest_json,
                    snapshot: selected.snapshot_json,
                  },
                  null,
                  2,
                )}
              </pre>
            </details>
          </article>
        ) : (
          <div className="panel empty">
            <p>
              Select a release to review its build and backend configuration.
            </p>
          </div>
        )}
      </div>
      {development && builds.length > 0 && (
        <details className="panel uploaded-builds">
          <summary>Uploaded builds</summary>
          {builds.map((b) => (
            <div key={b.id} className="development-heading">
              <span>
                {b.content_hash.slice(0, 12)} · {Math.ceil(b.size_bytes / 1024)}{" "}
                KB
              </span>
              <button disabled={busy} onClick={() => run(() => capture(b.id))}>
                Create release
              </button>
            </div>
          ))}
        </details>
      )}
      {development && (
        <p className="muted">
          Your agent can obtain the starter and perform uploads through MCP. The
          starter download contains files to write locally, pinned package
          versions and step-by-step instructions.
        </p>
      )}
      {modal && (
        <FormModal
          modal={modal}
          close={() => setModal(null)}
          done={() => setModal(null)}
          lookups={{}}
        />
      )}
    </section>
  );
}
