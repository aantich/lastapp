import { ExternalLink } from "lucide-react";
import { Alert, Button, Icon } from "./ui";
import { text, type Item } from "./api-types";
import { useApiQuery } from "./query";

// Marks every screen of a workspace's backing Space or App as development.
export function DevelopmentBanner({
  workspace,
  openSpace,
}: {
  workspace: string;
  openSpace: (id: string) => void;
}) {
  // The banner is decoration: without workspace details it still marks the
  // screen as development, so a failed read shows no error.
  const { data: info } = useApiQuery<Item>("/v1/workspaces/" + workspace);
  return (
    <Alert
      tone="info"
      live={false}
      className="development-banner"
      title={`Development workspace${info ? ` · ${text(info.display_name)}` : ""}`}
      actions={
        info && (
          <Button
            size="sm"
            icon={<Icon icon={ExternalLink} size="sm" aria-hidden />}
            onClick={() => openSpace(text(info.production_space_id))}
          >
            Production Space
          </Button>
        )
      }
    >
      Changes and test records are isolated from production.
    </Alert>
  );
}
