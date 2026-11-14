import { useEffect, useRef, useState } from "react";
import { Download, Upload, RefreshCw, ExternalLink } from "lucide-react";
import { api } from "./api";
import { useConsoleConfig } from "./console-config";
import { FormModal, type Modal } from "./form-modal";
import { status, type Item } from "./console-forms";
import "./development-workspaces.css";

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
  const [releases, setReleases] = useState<Item[]>([]),
    [builds, setBuilds] = useState<Item[]>([]),
    [selected, setSelected] = useState<Item>(),
    [deployment, setDeployment] = useState<Item>(),
    [error, setError] = useState(""),
    [deploymentError, setDeploymentError] = useState(""),
    [busy, setBusy] = useState(false),
    [reload, setReload] = useState(0),
    [modal, setModal] = useState<Modal | null>(null),
    [hasMore, setHasMore] = useState(false),
    [page, setPage] = useState(1);
  const { config } = useConsoleConfig();
  const refreshApp = useRef(refresh),
    previousDeployment = useRef<Item>(undefined);
  refreshApp.current = refresh;
  const folder = useRef<HTMLInputElement>(null),
    live = useRef(true),
    development = !!app.workspace_id;
  useEffect(() => {
    live.current = true;
    return () => {
      live.current = false;
    };
  }, []);
  useEffect(() => {
    let alive = true;
    setError("");
    Promise.all([
      api(`/v1/apps/${app.id}/releases?page=${page}`),
      ...(development ? [api(`/v1/apps/${app.id}/builds`)] : []),
    ])
      .then(([rels, b]) => {
        if (!alive) return;
        setReleases((old) =>
          page === 1
            ? rels.items
            : [
                ...old.filter(
                  (r) => !rels.items.some((n: Item) => n.id === r.id),
                ),
                ...rels.items,
              ],
        );
        setHasMore(!!rels.pagination?.has_more);
        setBuilds(b?.items || []);
      })
      .catch((e) => {
        if (alive) setError(e.message);
      });
    return () => {
      alive = false;
    };
  }, [app.id, tenant, development, reload, page]);
  useEffect(() => {
    let alive = true,
      timer: ReturnType<typeof setTimeout> | undefined;
    let retryDelay = 1500;
    async function poll() {
      try {
        const result = await api(`/v1/apps/${app.id}/deployment`);
        if (!alive) return;
        const latest: Item | undefined = result.deployment || undefined;
        const previous = previousDeployment.current;
        previousDeployment.current = latest;
        setDeployment(latest);
        setDeploymentError("");
        if (
          latest?.state === "active" &&
          previous && previous.id === latest.id &&
          previous.state !== "active"
        )
          refreshApp.current();
        retryDelay = 1500;
        if (
          !latest ||
          !["queued", "preparing", "publishing"].includes(latest.state)
        )
          return;
      } catch (e) {
        if (!alive) return;
        setDeploymentError(
          `Could not refresh deployment status: ${(e as Error).message}`,
        );
        if ([401, 403, 404].includes((e as { status?: number }).status || 0))
          return;
        retryDelay = Math.min(retryDelay * 2, 10000);
      }
      if (alive) timer = setTimeout(poll, retryDelay);
    }
    void poll();
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [app.id, reload]);
  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (e) {
      if (live.current) setError((e as Error).message);
    } finally {
      if (live.current) setBusy(false);
    }
  }
  async function upload(files: FileList | null) {
    if (!files?.length) return;
    await run(async () => {
      const ticket = await api(`/v1/apps/${app.id}/upload-tickets`, "POST", {}),
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
      setReload((x) => x + 1);
      await capture(result.id);
    });
    if (folder.current) folder.current.value = "";
  }
  async function capture(buildID: string) {
    const [latest, inventory] = await Promise.all([
      api(`/v1/tenants/${tenant}/apps/${app.id}`),
      api(`/v1/tenants/${tenant}/microapps?app_id=${app.id}&limit=200`),
    ]);
    const activeMicros: Item[] = inventory.items.filter(
      (m: Item) => m.state === "active",
    );
    if (
      inventory.pagination?.has_more ||
      activeMicros.length > config.development_policy.max_release_microapps
    )
      throw new Error("This App exceeds the release micro-app limit.");
    let previous: Item = {};
    if (releases[0])
      previous =
        (await api("/v1/releases/" + releases[0].id)).manifest_json || {};
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
        const rel = await api(
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
        setReload((x) => x + 1);
      },
    });
  }
  async function preview() {
    if (!selected) return;
    const current = await api(`/v1/tenants/${tenant}/apps/${app.id}`),
      result = await api(
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
          const target = await api(
            `/v1/tenants/${tenant}/apps/${selected.production_app_id}`,
          );
          revision = target.revision;
        } catch (e) {
          if ((e as { status?: number }).status !== 404) throw e;
        }
        const result = await api(
          `/v1/releases/${selected.id}/deploy`,
          "POST",
          {},
          revision,
        );
        previousDeployment.current = result;
        setDeployment(result);
        setReload((x) => x + 1);
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
            onClick={() => setReload((x) => x + 1)}
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
      {(error || deploymentError) && (
        <div className="error" role="alert">
          {error || deploymentError}
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
                run(async () => setSelected(await api("/v1/releases/" + r.id)))
              }
            >
              <strong>{r.display_name}</strong>
              <small>{new Date(r.created_at).toLocaleString()}</small>
            </button>
          ))}
          {hasMore && (
            <button onClick={() => setPage((p) => p + 1)}>Load more</button>
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
          done={() => {
            setModal(null);
            setReload((x) => x + 1);
          }}
          lookups={{}}
        />
      )}
    </section>
  );
}
