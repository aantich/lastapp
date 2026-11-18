import { test, expect } from "@playwright/test";
import {
  login,
  logout,
  fixtureTenant,
  selectTenant,
  post,
  menu,
} from "./helpers";
test("entity designer creates, tests and publishes a real shared backend", async ({
  page,
}) => {
  async function capture(name: string, fullPage = true) {
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: "test-results/" + name,
      fullPage,
      animations: "disabled",
    });
  }
  await login(page);
  const { tenant, owner, base } = await fixtureTenant(page, "Entity designer");
  await logout(page);
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  const space = crypto.randomUUID();
  const created = await post(page, base + "/spaces", {
    id: space,
    slug: "crm",
    display_name: "CRM",
    initial_manager_user_id: owner.id,
    database: {
      mode: "managed",
      layout: "shared",
      target_id: "20000000-0000-4000-8000-000000000001",
    },
  });
  expect(created.status).toBe(201);
  await page.goto(
    "/console/tenants/" + tenant + "/spaces/" + space + "/entities",
  );
  await expect(
    page.getByRole("heading", { name: "CRM entities", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "New entity", exact: true }).click();
  await page.getByLabel("Entity name", { exact: true }).fill("Deal");
  await page
    .getByRole("button", { name: "Create entity", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Deal", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Add field", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name", { exact: true }).fill("Name");
  await dialog.getByLabel("Required", { exact: true }).check();
  await dialog.getByRole("button", { name: "Save field", exact: true }).click();
  await page.getByRole("button", { name: "Add field", exact: true }).click();
  await dialog.getByLabel("Name", { exact: true }).fill("Website");
  await dialog.getByRole("button", { name: "Save field", exact: true }).click();
  const draftSaved = page.waitForResponse(
    (r) =>
      r.request().method() === "PATCH" &&
      /\/v1\/entities\/[a-f0-9-]+$/.test(new URL(r.url()).pathname) &&
      r.status() === 200,
  );
  await page.getByRole("button", { name: "Save draft", exact: true }).click();
  await draftSaved;
  const entities = await (
    await page.request.get("/v1/spaces/" + space + "/entities")
  ).json();
  const id = entities.items[0].id;
  let saved = await (await page.request.get("/v1/entities/" + id)).json();
  // Agent authoring uses the same document as the visual editor.
  const definition = saved.draft_json;
  definition.default_projection = ["name"];
  definition.permissions = [
    {
      key: "viewer",
      label: "View deals",
      operation: "read",
      relationship: "member",
      fields: ["name"],
    },
    {
      key: "creator",
      label: "Create deals",
      operation: "create",
      relationship: "member",
      fields: ["name"],
    },
  ];
  expect(
    (
      await post(
        page,
        "/v1/entities/" + id,
        { definition },
        "PATCH",
        saved.revision,
      )
    ).status,
  ).toBe(200);
  await page.reload();

  await expect(
    page.getByRole("heading", { name: "Deal", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("tablist", { name: "Entity sections" })
    .getByRole("tab", { name: "Access", exact: true })
    .click();
  await expect(page.getByText("View deals", { exact: true })).toBeVisible();
  await page
    .getByRole("tablist", { name: "Entity sections" })
    .getByRole("tab", { name: "Test", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "Name", exact: true })
    .fill("Synthetic Deal");
  await page.getByRole("button", { name: "Run test", exact: true }).click();
  await expect(page.getByText("Allowed", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Allowed through Create deals.", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Review & publish", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Publish", exact: true })
    .click();
  await expect
    .poll(
      async () => {
        saved = await (await page.request.get("/v1/entities/" + id)).json();
        return saved.publications[0]?.state;
      },
      { timeout: 30000 },
    )
    .toBe("published");
  await expect(page.getByText(/^deal · published /)).toBeVisible();
  // The Space list flags no unpublished changes once the draft is live.
  await expect(
    page
      .getByRole("navigation", { name: "CRM entities", exact: true })
      .getByText("unpublished changes"),
  ).toHaveCount(0);
  await page
    .getByRole("tablist", { name: "Entity sections" })
    .getByRole("tab", { name: "Fields", exact: true })
    .click();
  await page.getByText("Entity settings", { exact: true }).click();
  const nameInput = await page
    .getByRole("textbox", { name: "Name", exact: true })
    .boundingBox();
  const ownerInput = await page
    .getByRole("combobox", { name: "Owner field", exact: true })
    .boundingBox();
  const teamInput = await page
    .getByRole("combobox", { name: "Owning team field", exact: true })
    .boundingBox();
  expect(Math.abs(ownerInput!.y - teamInput!.y)).toBeLessThanOrEqual(1);
  expect(Math.abs(nameInput!.y - ownerInput!.y)).toBeLessThanOrEqual(1);
  await page.setViewportSize({ width: 1920, height: 1080 });
  await capture("entity-designer-desktop.png");
  await menu(page);
  await page.getByRole("menuitem", { name: "Appearance", exact: true }).click();
  const appearance = page.getByRole("dialog", {
    name: "Appearance",
    exact: true,
  });
  await appearance.getByRole("radio", { name: "Dark", exact: true }).check();
  await appearance.getByRole("button", { name: "Done", exact: true }).click();
  await capture("entity-designer-dark.png");
  await menu(page);
  await page.getByRole("menuitem", { name: "Appearance", exact: true }).click();
  await appearance.getByRole("button", { name: "Reset to default" }).click();
  await appearance.getByRole("button", { name: "Done", exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(
    page.getByRole("button", { name: "Review & publish", exact: true }),
  ).toBeVisible();
  await expect(page.locator("aside.sidebar")).toHaveCSS("visibility", "hidden");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await capture("entity-designer-mobile.png");
  await page.getByRole("button", { name: "Edit Website", exact: true }).click();
  await expect(dialog).toBeVisible();
  const editor = await dialog.boundingBox();
  expect(editor!.x).toBeGreaterThanOrEqual(0);
  expect(editor!.x + editor!.width).toBeLessThanOrEqual(390);
  await capture("entity-editor-mobile.png", false);
  await page.getByRole("button", { name: "Close editor", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Edit Website", exact: true }),
  ).toBeFocused();
  // Only publication status is simulated here; both editors write the real registry.
  // A background refresh must never advance a dirty editor over another draft.
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.route("**/v1/entities/" + id, async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    const response = await route.fetch();
    const value = await response.json();
    value.pending_version_id = crypto.randomUUID();
    if (value.draft_json.label === "Remote editor") {
      value.publications = [
        { state: "failed", error_message: "Concurrent refresh observed." },
      ];
    }
    await route.fulfill({ response, json: value });
  });
  await page.reload();
  await page.getByText("Entity settings", { exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "Name", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Name", exact: true })
    .fill("Local unsaved editor");
  const remote = await (await page.request.get("/v1/entities/" + id)).json();
  expect(
    (
      await post(
        page,
        "/v1/entities/" + id,
        { definition: { ...remote.draft_json, label: "Remote editor" } },
        "PATCH",
        remote.revision,
      )
    ).status,
  ).toBe(200);
  await expect(page.getByText(/Concurrent refresh observed/)).toBeVisible();
  await page.getByRole("button", { name: "Save draft", exact: true }).click();
  await expect(
    page.getByText("This resource changed. Refresh before saving.", {
      exact: true,
    }),
  ).toBeVisible();
  expect(
    (await (await page.request.get("/v1/entities/" + id)).json()).draft_json
      .label,
  ).toBe("Remote editor");
  await expect(
    page.getByRole("textbox", { name: "Name", exact: true }),
  ).toHaveValue("Local unsaved editor");
  await page.unroute("**/v1/entities/" + id);
});
