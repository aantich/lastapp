import { Pencil } from "lucide-react";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Disclosure,
  FactList,
  StatusBadge,
} from "../ui";
import { text } from "../api-types";
import { computeField, editFields, type TenantScreen } from "./screen";

// The App's name, status and compute preference.
export function AppSettings({ t }: { t: TenantScreen }) {
  const { app, base, edit, profile, scoped } = t;
  return (
    <Card>
      <CardHeader
        title="General"
        actions={
          <>
            <StatusBadge state={app.state} />
            {scoped("app.manage") && (
              <Button
                icon={<Pencil size={14} aria-hidden />}
                onClick={() =>
                  edit(app, base + "/apps/" + app.id, "Edit App", [
                    ...editFields,
                    computeField(
                      !!profile.allow_dedicated_compute ||
                        app.compute_mode === "dedicated",
                    ),
                  ])
                }
              >
                Edit App
              </Button>
            )}
          </>
        }
      />
      <CardBody>
        <FactList
          facts={[
            { label: "Name", value: text(app.display_name) },
            { label: "Space", value: text(app.space_name) },
          ]}
        />
        <Disclosure summary="Advanced">
          <FactList
            facts={[
              { label: "Slug", value: text(app.slug), mono: true },
              {
                label: "Compute preference",
                value: text(app.compute_mode),
              },
            ]}
          />
        </Disclosure>
      </CardBody>
    </Card>
  );
}
