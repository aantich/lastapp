import { useEffect, useRef, useState } from "react";
import {
  Download,
  ExternalLink,
  RefreshCw,
  Rocket,
  RotateCcw,
  Upload,
} from "lucide-react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { api, type Failure } from "./api";
import { apiKey, queryClient, refreshQueries, useApiQuery } from "./query";
import { useConsoleConfig } from "./console-config";
import { deploymentSteps, useDeploymentStates } from "./deployment";
import {
  Alert,
  Button,
  Card,
  CardBody,
  CardHeader,
  CodeBlock,
  DataTable,
  Disclosure,
  EmptyState,
  FactList,
  FormModal,
  IconButton,
  LinkButton,
  PickList,
  Split,
  Stack,
  StatusBadge,
  Stepper,
  UploadButton,
  type Modal,
  type Step,
} from "./ui";
import {
  keyOf,
  num,
  text,
  type Build,
  type Deployment,
  type Item,
  type Page,
  type Release,
  type ReleaseManifest,
  type UploadTicket,
} from "./api-types";
import { errorMessage, useAction } from "./use-action";

async function encode(file: File) {
  const bytes = new Uint8Array(await file.arrayBuffer());
  let text = "";
  for (let i = 0; i < bytes.length; i += 8192)
    text += String.fromCharCode(...bytes.subarray(i, i + 8192));
  return btoa(text);
}
const when = (at: unknown) => new Date(text(at)).toLocaleString();

// An App's Releases tab: the ship flow. In a development workspace App the
// author uploads a build, captures it as an immutable release, previews it
// and deploys it; a production App shows what it runs and where to develop.
// The production deployment card follows the real deployment states.
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
  const [selected, setSelected] = useState<Release>(),
    [error, setError] = useState(""),
    [modal, setModal] = useState<Modal | null>(null);
  const { config } = useConsoleConfig();
  const states = useDeploymentStates();
  const development = !!app.workspace_id;
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
    useApiQuery<Page<Build>>(development ? `/v1/apps/${app.id}/builds` : null)
      .data?.items || [];
  // Production deployment status: poll while it is running, backing off
  // from 1.5 s to 10 s after failures, and stop at a final state or when
  // access is gone. refetchInterval never overlaps requests.
  const deploymentPath = `/v1/apps/${app.id}/deployment`,
    failures = useRef({ at: 0, count: 0 });
  const deploymentQuery = useApiQuery<{ deployment?: Deployment | null }>(
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
        return states.running(data?.deployment?.state) ? 1500 : false;
      },
    },
  );
  const deployment = deploymentQuery.data?.deployment || undefined,
    deploymentError = deploymentQuery.error
      ? `Could not refresh deployment status: ${errorMessage(deploymentQuery.error)}`
      : "";
  // When this deployment goes live, the App itself changed: refresh it.
  const previousDeployment = useRef<Deployment | undefined>(undefined);
  useEffect(() => {
    const previous = previousDeployment.current;
    previousDeployment.current = deployment;
    if (
      deployment?.state === states.live &&
      previous &&
      previous.id === deployment.id &&
      previous.state !== states.live
    )
      refresh();
  }, [deployment, refresh, states.live]);
  const { busy, run } = useAction(app.id, {
    onSuccess: () => {},
    onError: setError,
  });
  async function upload(files: FileList) {
    await run(async () => {
      const ticket = await api<UploadTicket>(
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
    let previous: ReleaseManifest = {};
    if (releases[0])
      previous =
        (await api<Release>("/v1/releases/" + releases[0].id)).manifest_json ||
        {};
    setModal({
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
        num(current.revision),
      );
    refresh();
    window.open(result.url, "_blank", "noopener,noreferrer");
  }
  // Deploys a release to production: a new deployment of the same release
  // is also how a failed one is retried (POST /v1/releases/{id}/deploy).
  function deploy(release: Release, retry = false) {
    setModal({
      title: (retry ? "Retry deploy of " : "Deploy ") + release.display_name,
      description:
        (retry ? "Starts a new deployment of this release. " : "") +
        "Applies the captured shared Space backend configuration, then activates this App build. Other Apps reuse those Space entities and permissions. Database changes are not automatically rolled back if a step fails.",
      success: "Deployment queued",
      initial: {},
      fields: [],
      submit: retry ? "Retry deploy" : "Deploy to production",
      save: async () => {
        let revision: number | undefined = 1;
        try {
          const target = await api<Item>(
            `/v1/tenants/${tenant}/apps/${release.production_app_id}`,
          );
          revision = num(target.revision);
        } catch (e) {
          if ((e as { status?: number }).status !== 404) throw e;
        }
        const result = await api<Deployment>(
          `/v1/releases/${release.id}/deploy`,
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
  const deployed = deployment
    ? releases.find((r) => r.id === deployment.release_id)
    : undefined;
  const failed = deployment?.state === states.failed,
    live = deployment?.state === states.live;
  const refreshButton = (
    <IconButton
      label="Refresh releases"
      icon={<RefreshCw size={16} aria-hidden />}
      tooltip="bottom"
      disabled={busy}
      onClick={() => void refreshQueries()}
    />
  );
  // Where this development App is on the way to production.
  const ship: Step[] = [
    {
      id: "workspace",
      label: "Workspace",
      note: "Isolated from production",
      state: "done",
    },
    {
      id: "build",
      label: "Upload a build",
      note: builds.length
        ? `${builds.length} uploaded`
        : "Build locally, then upload the folder",
      state: builds.length ? "done" : "active",
    },
    {
      id: "release",
      label: "Create a release",
      note: releases[0]
        ? `Latest: ${text(releases[0].display_name)}`
        : "Pins the build, micro-apps and Space snapshot",
      state: releases.length ? "done" : builds.length ? "active" : "pending",
    },
    {
      id: "deploy",
      label: "Deploy to production",
      note: deployment ? `Latest deployment: ${deployment.state}` : undefined,
      state: !deployment
        ? "pending"
        : live
          ? "done"
          : failed
            ? "failed"
            : "active",
    },
  ];
  return (
    <Stack>
      {(error || listError || deploymentError) && (
        <Alert>{error || listError || deploymentError}</Alert>
      )}
      {development ? (
        <Card>
          <CardHeader
            title="Ship this App"
            description="Build locally, test this workspace, then deploy a reviewed release."
            actions={
              <>
                {refreshButton}
                <LinkButton
                  href={`/v1/apps/${app.id}/starter.zip`}
                  icon={<Download size={16} aria-hidden />}
                >
                  Get starter
                </LinkButton>
                <UploadButton
                  directory
                  variant="primary"
                  loading={busy}
                  icon={<Upload size={16} aria-hidden />}
                  onFiles={upload}
                >
                  {busy ? "Working…" : "Upload build folder"}
                </UploadButton>
              </>
            }
          />
          <CardBody>
            <Stack gap="md">
              <Stepper label="Ship flow" steps={ship} />
              <Alert tone="info" live={false}>
                Your agent can obtain the starter and perform uploads through
                MCP. The starter download contains files to write locally,
                pinned package versions and step-by-step instructions.
              </Alert>
            </Stack>
          </CardBody>
        </Card>
      ) : (
        <Card>
          <CardHeader
            title="Ship changes"
            description="Production runs a selected immutable release. Develop changes in an isolated workspace, then deploy a release from it."
            actions={
              <>
                {refreshButton}
                <Button variant="primary" onClick={openWorkspaces}>
                  Open development workspaces
                </Button>
              </>
            }
          />
        </Card>
      )}
      <Card className="deployment-status" role="status">
        <CardHeader
          title="Production deployment"
          description={
            !deployment
              ? undefined
              : failed
                ? "The previous release stays selected in production."
                : live
                  ? `${deployed ? text(deployed.display_name) : "This release"} is live.`
                  : "Production keeps serving the previous release until this one goes live."
          }
          actions={
            deployment && (
              <>
                <StatusBadge state={deployment.state} />
                {live && (
                  <LinkButton
                    href={deployment.url}
                    external
                    size="sm"
                    trailingIcon={<ExternalLink size={14} aria-hidden />}
                  >
                    Open production App
                  </LinkButton>
                )}
                {failed && deployment.release_id && (
                  <Button
                    size="sm"
                    variant="primary"
                    icon={<RotateCcw size={14} aria-hidden />}
                    disabled={busy}
                    onClick={() =>
                      run(async () =>
                        deploy(
                          await api<Release>(
                            "/v1/releases/" + deployment.release_id,
                          ),
                          true,
                        ),
                      )
                    }
                  >
                    Retry deploy
                  </Button>
                )}
              </>
            )
          }
        />
        <CardBody>
          {deployment ? (
            <Stack gap="md">
              {failed ? (
                <Alert as="p" live={false} title="Deployment failed">
                  {deployment.error_message}
                </Alert>
              ) : (
                <Stepper
                  label="Deployment progress"
                  steps={deploymentSteps(states.steps, deployment.state)}
                />
              )}
              <FactList
                facts={[
                  {
                    label: "Release",
                    value: deployed
                      ? text(deployed.display_name)
                      : text(deployment.release_id).slice(0, 8),
                    mono: !deployed,
                  },
                  ...(deployment.created_at
                    ? [{ label: "Started", value: when(deployment.created_at) }]
                    : []),
                  ...(deployment.finished_at
                    ? [
                        {
                          label: "Finished",
                          value: when(deployment.finished_at),
                        },
                      ]
                    : []),
                ]}
              />
            </Stack>
          ) : (
            <EmptyState compact title="Not deployed yet">
              Deploy a release to put this App in production.
            </EmptyState>
          )}
        </CardBody>
      </Card>
      <Split
        aside={
          <Card className="release-list">
            <CardHeader
              title="Releases"
              description="Immutable build, micro-app pins and Space snapshot"
            />
            <CardBody>
              <PickList
                label="Release history"
                itemClassName="release-item"
                disabled={busy}
                value={selected?.id}
                empty="No releases yet."
                items={releases.map((r) => ({
                  id: keyOf(r.id) ?? "",
                  title: text(r.display_name),
                  meta: when(r.created_at),
                  badge:
                    deployment && deployment.release_id === r.id ? (
                      <StatusBadge
                        state={deployment.state}
                        label={live ? "Production" : undefined}
                      />
                    ) : undefined,
                }))}
                onPick={(id) =>
                  run(async () =>
                    setSelected(await api<Release>("/v1/releases/" + id)),
                  )
                }
                footer={
                  hasMore && (
                    <Button
                      size="sm"
                      onClick={() => void releasePages.fetchNextPage()}
                    >
                      Load more
                    </Button>
                  )
                }
              />
            </CardBody>
          </Card>
        }
      >
        {selected ? (
          <ReleaseReview
            release={selected}
            busy={busy}
            preview={development ? () => run(preview) : undefined}
            deploy={() => deploy(selected)}
          />
        ) : (
          <Card>
            <EmptyState title="No release open">
              Select a release to review its build and backend configuration.
            </EmptyState>
          </Card>
        )}
      </Split>
      {development && builds.length > 0 && (
        <Card>
          <CardHeader
            title="Uploaded builds"
            description="Create a release from an earlier upload."
          />
          <DataTable
            caption="Uploaded builds"
            rows={builds}
            rowKey={(b) => b.id}
            columns={[
              {
                key: "build",
                header: "Build",
                render: (b) => <code>{b.content_hash.slice(0, 12)}</code>,
              },
              {
                key: "size",
                header: "Size",
                numeric: true,
                render: (b) => `${Math.ceil(b.size_bytes / 1024)} KB`,
              },
              {
                key: "uploaded",
                header: "Uploaded",
                render: (b) => when(b.created_at),
              },
            ]}
            actions={(b) => (
              <Button
                size="sm"
                disabled={busy}
                onClick={() => run(() => capture(b.id))}
              >
                Create release
              </Button>
            )}
          />
        </Card>
      )}
      {modal && (
        <FormModal
          modal={modal}
          close={() => setModal(null)}
          done={() => setModal(null)}
          lookups={{}}
        />
      )}
    </Stack>
  );
}

// One immutable release: its build, pins and captured Space backend.
function ReleaseReview({
  release,
  busy,
  preview,
  deploy,
}: {
  release: Release;
  busy: boolean;
  preview?: () => void;
  deploy: () => void;
}) {
  const snapshot = release.snapshot_json,
    pins = Object.entries(release.manifest_json?.microapps || {}),
    packages = Object.keys(release.manifest_json?.dependencies || {});
  return (
    <Card as="article">
      <CardHeader
        title={release.display_name}
        description={`Immutable build · ${release.build_id.slice(0, 8)}`}
        actions={
          <>
            {preview && (
              <Button
                disabled={busy}
                icon={<ExternalLink size={14} aria-hidden />}
                onClick={preview}
              >
                Open preview
              </Button>
            )}
            <Button
              variant="primary"
              disabled={busy}
              icon={<Rocket size={16} aria-hidden />}
              onClick={deploy}
            >
              Deploy to production
            </Button>
          </>
        }
      />
      <CardBody>
        <Stack gap="md">
          <FactList
            facts={[
              { label: "Build", value: release.build_id, mono: true },
              { label: "Created", value: when(release.created_at) },
              {
                label: "Micro-apps",
                value: pins.length
                  ? pins.map(([slug, v]) => `${slug} ${v}`).join(", ")
                  : "None pinned",
              },
              {
                label: "Shared packages",
                value: packages.length
                  ? `${packages.length} pinned`
                  : "None pinned",
              },
              {
                label: "Captured Space backend",
                value: `${snapshot.entities.length} entities · ${snapshot.roles.length} business roles · ${snapshot.apps.length} Apps`,
              },
            ]}
          />
          <Stack gap="sm">
            {snapshot.entities.map((e) => (
              <Disclosure
                key={e.id}
                summary={e.definition.label}
                meta={`${e.definition.fields.length} fields · ${e.definition.permissions.length} permissions`}
              >
                <DataTable
                  caption={`${e.definition.label} fields`}
                  rows={e.definition.fields}
                  rowKey={(f) => keyOf(f.key)}
                  columns={[
                    {
                      key: "field",
                      header: "Field",
                      render: (f) => <strong>{text(f.label)}</strong>,
                    },
                    {
                      key: "type",
                      header: "Type",
                      render: (f) => <code>{text(f.type)}</code>,
                    },
                    {
                      key: "required",
                      header: "Required",
                      render: (f) => (f.required ? "Required" : "Optional"),
                    },
                  ]}
                />
              </Disclosure>
            ))}
            <Disclosure summary="Manifest and full configuration">
              <CodeBlock label={`${release.display_name} configuration`}>
                {JSON.stringify(
                  { manifest: release.manifest_json, snapshot },
                  null,
                  2,
                )}
              </CodeBlock>
            </Disclosure>
          </Stack>
        </Stack>
      </CardBody>
    </Card>
  );
}
