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
import { api } from "./api";
import { apiKey, queryClient, refreshQueries, useApiQuery } from "./query";
import { useConsoleConfig } from "./console-config";
import {
  deployModal,
  deploymentSteps,
  retryModal,
  useDeploymentStates,
} from "./deployment";
import { ago } from "./actors";
import {
  Alert,
  Button,
  Card,
  CardBody,
  CardHeader,
  CellActions,
  CellTitle,
  CodeBlock,
  DataTable,
  Disclosure,
  FactList,
  FormModal,
  IconButton,
  LinkButton,
  PageStack,
  RowTable,
  Stack,
  StatusBadge,
  Stepper,
  TitledCard,
  UploadButton,
  type Modal,
  type RowTableRow,
  type Step,
} from "./ui";
import {
  keyOf,
  num,
  optText,
  text,
  type Build,
  type Deployment,
  type Item,
  type Page,
  type Release,
  type ReleaseItem,
  type ReleaseManifest,
  type UploadTicket,
} from "./api-types";
import { useAction } from "./use-action";

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
  deployment,
  deploymentError,
  refresh,
}: {
  app: Item;
  tenant: string;
  deployment: Deployment | null | undefined;
  deploymentError: string;
  refresh: () => void;
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
      api<Page<ReleaseItem>>(
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
  const releases = (releasePages.data?.pages || []).reduce<ReleaseItem[]>(
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
  const deploymentPath = `/v1/apps/${app.id}/deployment`;
  // When this deployment goes live, the App itself changed: refresh it.
  const previousDeployment = useRef<Deployment | undefined>(undefined);
  useEffect(() => {
    const previous = previousDeployment.current;
    previousDeployment.current = deployment || undefined;
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
  async function preview(release: string) {
    const current = await api<Item>(`/v1/tenants/${tenant}/apps/${app.id}`),
      result = await api<{ url: string }>(
        `/v1/releases/${release}/preview`,
        "POST",
        {},
        num(current.revision),
      );
    refresh();
    window.open(result.url, "_blank", "noopener,noreferrer");
  }
  // Show a new or retried deployment at once; its state restarts the polling.
  function show(result: Deployment) {
    previousDeployment.current = result;
    queryClient.setQueryData(apiKey(deploymentPath), { deployment: result });
  }
  // Deploys a release to production.
  function deploy(release: Release) {
    setModal(deployModal(tenant, release, show));
  }
  const failed = deployment?.state === states.failed,
    live = deployment?.state === states.live;
  // A failed release was never active, so the production App's list (only
  // releases that went live there) does not have it: show it from the
  // deployment.
  const listed: ReleaseItem[] =
    deployment &&
    !development &&
    !releases.some((r) => r.id === deployment.release_id)
      ? [
          {
            id: deployment.release_id,
            display_name: deployment.release_name,
            build_id: "",
            created_at: deployment.created_at,
            created_by_name: deployment.deployed_by,
            agent_client_name: null,
            snapshot_hash: "",
          },
          ...releases,
        ]
      : releases;
  const serving = optText(app.active_release_id);
  const servingName = listed.find((r) => r.id === serving)?.display_name;
  const name = deployment?.release_name || "This release";
  const note = !deployment
    ? "Not deployed yet. Deploy a release to put this App in production."
    : live
      ? `${name} active since ${time(deployment.finished_at || deployment.created_at)}`
      : failed
        ? servingName && !development
          ? `Production still serves ${servingName}.`
          : "Production is unchanged."
        : servingName && !development
          ? `Production keeps serving ${servingName} until ${name} is active.`
          : `Production keeps serving its current release until ${name} is active.`;
  const steps = deployment ? deploymentSteps(states.steps, deployment) : [];
  function open(id: string) {
    return run(async () =>
      setSelected(await api<Release>("/v1/releases/" + id)),
    );
  }
  // A release's place: what production serves, what is deploying or
  // failed, earlier production releases, and in a workspace what preview
  // serves.
  function chip(r: ReleaseItem) {
    if (deployment?.release_id === r.id && !live)
      return failed ? (
        <StatusBadge state={deployment.state} label="Failed" />
      ) : (
        <StatusBadge
          state={deployment.state}
          tone="warning"
          label="Deploying"
        />
      );
    if (development)
      return deployment?.release_id === r.id ? (
        <StatusBadge state={states.live} label="Production" />
      ) : serving === r.id ? (
        <StatusBadge state="preview" tone="info" label="In preview" />
      ) : null;
    return serving === r.id ? (
      <StatusBadge state={states.live} label="Production" />
    ) : (
      <StatusBadge state="previous" tone="neutral" label="Previous" />
    );
  }
  const rows: RowTableRow[] = listed.map((r) => {
    const author = r.agent_client_name
      ? `${r.agent_client_name} for ${r.created_by_name}`
      : r.created_by_name;
    return {
      id: r.id,
      cells: [
        <CellTitle
          title={r.display_name}
          meta={[ago(r.created_at), author].filter(Boolean).join(" · ")}
        />,
        chip(r),
        <CellActions>
          {development && (
            <Button
              size="sm"
              disabled={busy}
              aria-label={`Preview ${r.display_name}`}
              onClick={() => run(() => preview(r.id))}
            >
              Preview
            </Button>
          )}
          <Button
            size="sm"
            disabled={busy}
            aria-label={`${r.display_name} details`}
            onClick={() => open(r.id)}
          >
            Details
          </Button>
        </CellActions>,
      ],
    };
  });
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
        ? `Latest: ${releases[0].display_name}`
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
    <PageStack>
      {(error || listError || deploymentError) && (
        <Alert>{error || listError || deploymentError}</Alert>
      )}
      {development && (
        <Card>
          <CardHeader
            title="Ship this App"
            description="Build locally, test this workspace, then deploy a reviewed release."
            actions={
              <>
                <IconButton
                  label="Refresh releases"
                  icon={<RefreshCw size={16} aria-hidden />}
                  tooltip="bottom"
                  disabled={busy}
                  onClick={() => void refreshQueries()}
                />
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
      )}
      <TitledCard
        className="deployment-status"
        role="status"
        title="Production deployment"
        note={deployment === undefined ? "Loading…" : note}
        actions={
          deployment &&
          (failed ? (
            <Button
              size="sm"
              variant="primary"
              icon={<RotateCcw size={14} aria-hidden />}
              disabled={busy}
              onClick={() => setModal(retryModal(deployment, show))}
            >
              Retry deploy
            </Button>
          ) : (
            live &&
            development && (
              <LinkButton
                href={deployment.url}
                external
                size="sm"
                trailingIcon={<ExternalLink size={14} aria-hidden />}
              >
                Open production App
              </LinkButton>
            )
          ))
        }
      >
        {deployment &&
          (steps.length ? (
            <Stepper label="Deployment progress" steps={steps} min={160} />
          ) : (
            failed && (
              <Alert as="p" live={false} title="Deployment failed">
                {deployment.error_message}
              </Alert>
            )
          ))}
      </TitledCard>
      <RowTable
        className="release-list"
        rowClassName="release-item"
        label="Releases"
        title="Releases"
        note="Immutable build + micro-app pins + Space snapshot"
        columns="minmax(200px, 2fr) 150px auto"
        rows={rows}
        empty={releasePages.data ? "No releases yet." : "Loading…"}
        footer={
          hasMore && (
            <Button size="sm" onClick={() => void releasePages.fetchNextPage()}>
              Load more
            </Button>
          )
        }
      />
      {selected && (
        <ReleaseReview
          release={selected}
          busy={busy}
          preview={
            development ? () => run(() => preview(selected.id)) : undefined
          }
          deploy={() => deploy(selected)}
        />
      )}
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
    </PageStack>
  );
}

// "09:41" today, otherwise the date and time, in the reader's locale.
function time(at: string) {
  const d = new Date(at);
  return d.toDateString() === new Date().toDateString()
    ? d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })
    : d.toLocaleString(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      });
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
