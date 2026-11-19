import { ExternalLink, Rocket } from "lucide-react";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  CodeBlock,
  DataTable,
  Disclosure,
  FactList,
  Stack,
} from "../ui";
import { keyOf, text, type Release } from "../api-types";
import { when } from "./release-dates";

// One immutable release: its build, pins and captured Space backend.
export function ReleaseReview({
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
