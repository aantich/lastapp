import { Download, RefreshCw, Upload } from "lucide-react";
import { refreshQueries } from "../query";
import {
  Alert,
  Card,
  CardBody,
  CardHeader,
  IconButton,
  LinkButton,
  Stack,
  Stepper,
  UploadButton,
  type Step,
} from "../ui";
import type { Build, Deployment, ReleaseItem } from "../api-types";

// A development workspace App's ship flow: get the starter, upload a build
// and follow it on the way to production.
export function ShipCard({
  app,
  busy,
  builds,
  releases,
  deployment,
  live,
  failed,
  upload,
}: {
  app: unknown;
  busy: boolean;
  builds: Build[];
  releases: ReleaseItem[];
  deployment: Deployment | null | undefined;
  live: boolean;
  failed: boolean;
  upload: (files: FileList) => Promise<void>;
}) {
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
              href={`/v1/apps/${app}/starter.zip`}
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
            Your agent can obtain the starter and perform uploads through MCP.
            The starter download contains files to write locally, pinned package
            versions and step-by-step instructions.
          </Alert>
        </Stack>
      </CardBody>
    </Card>
  );
}
