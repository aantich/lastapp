import { test, expect } from "@playwright/test";
import { login, logout, fixtureTenant, choose, confirmDialog } from "./helpers";
import { apiOrigin } from "./stack";

test("OAuth consent scopes an agent and account settings edit and revoke it", async ({
  page,
}) => {
  await login(page);
  const { tenant } = await fixtureTenant(page, "Agent access");
  await logout(page);
  await login(page, "tenant-admin");
  const challenge = await page.evaluate(async () => {
    const bytes = await crypto.subtle.digest(
      "SHA-256",
      new TextEncoder().encode("v".repeat(64)),
    );
    return btoa(String.fromCharCode(...new Uint8Array(bytes)))
      .replaceAll("+", "-")
      .replaceAll("/", "_")
      .replaceAll("=", "");
  });
  const registration = await page.request.post("/oauth/register", {
    data: {
      client_name: "UI test agent",
      redirect_uris: ["http://127.0.0.1:8790/callback"],
      token_endpoint_auth_method: "none",
      grant_types: ["authorization_code", "refresh_token"],
      response_types: ["code"],
    },
  });
  expect(registration.status()).toBe(201);
  const client = await registration.json();
  const oauth = new URL(apiOrigin + "/oauth/authorize");
  oauth.search = new URLSearchParams({
    response_type: "code",
    client_id: client.client_id,
    redirect_uri: "http://127.0.0.1:8790/callback",
    resource: apiOrigin + "/mcp",
    code_challenge_method: "S256",
    code_challenge: challenge,
    state: "ui-agent-state",
  }).toString();
  await page.goto(oauth.toString());
  await expect(
    page.getByRole("heading", { name: "Connect an agent" }),
  ).toBeVisible();
  await expect(page.getByText("Returns to 127.0.0.1:8790")).toBeVisible();
  // Presets are cards; single permissions are under a disclosure.
  await page.getByText("Choose individual permissions").click();
  await expect(
    page.getByRole("checkbox", { name: /Create and edit resources/ }),
  ).toBeVisible();
  await expect(
    page.getByRole("checkbox", { name: /Create and edit resources/ }),
  ).not.toBeChecked();
  await choose(page, "Tenant", tenant);
  await page
    .getByRole("button", { name: "Entity author", exact: true })
    .click();
  await expect(
    page.getByRole("checkbox", { name: /Publish entities/ }),
  ).not.toBeChecked();
  expect(
    await page
      .locator(
        '.agent-grant-form input:not([type="checkbox"]), .agent-grant-form .select-trigger, .agent-grant-form .ui-date-trigger',
      )
      .evaluateAll((nodes) =>
        nodes.every((node) => node.getBoundingClientRect().height >= 36),
      ),
  ).toBe(true);
  await page.screenshot({
    path: "test-results/mcp-consent-desktop.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator(".agent-consent-card")).toBeInViewport();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "test-results/mcp-consent-mobile.png",
    fullPage: true,
  });
  await page.route("http://127.0.0.1:8790/callback**", (route) =>
    route.fulfill({
      contentType: "text/html",
      body: "<h1>Agent connected</h1>",
    }),
  );
  await page.getByRole("button", { name: "Approve connection" }).click();
  await expect(
    page.getByRole("heading", { name: "Agent connected" }),
  ).toBeVisible();
  const returned = new URL(page.url());
  expect(returned.searchParams.get("state")).toBe("ui-agent-state");
  expect(returned.searchParams.get("code")).toBeTruthy();
  const exchange = await page.request.post("/oauth/token", {
    form: {
      grant_type: "authorization_code",
      code: returned.searchParams.get("code")!,
      client_id: client.client_id,
      redirect_uri: "http://127.0.0.1:8790/callback",
      resource: apiOrigin + "/mcp",
      code_verifier: "v".repeat(64),
    },
  });
  expect(exchange.status()).toBe(200);
  const tokens = await exchange.json();
  expect(tokens.scope.split(" ").sort()).toEqual([
    "entities.check",
    "entities.draft",
    "read",
  ]);
  await page.goto(apiOrigin + "/console/account");
  await expect(
    page.getByRole("heading", { name: "Agent access", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Copy MCP URL" }),
  ).toBeVisible();
  const grant = page
    .locator(".agent-connection")
    .filter({ hasText: "UI test agent" })
    .first();
  await expect(grant).toBeVisible();
  await page.evaluate(() => {
    Object.defineProperty(navigator.clipboard, "writeText", {
      configurable: true,
      value: () => Promise.reject(new Error("Clipboard unavailable")),
    });
  });
  await page.getByRole("button", { name: "Copy MCP URL" }).click();
  await expect(page.getByRole("alert")).toContainText("Select and copy");
  await expect(grant).toBeVisible();
  await expect(
    grant.getByRole("button", { name: "Revoke", exact: true }),
  ).toBeEnabled();
  await grant.getByRole("button", { name: "Edit", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  const bounds = await dialog.boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(8);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(382);
  await dialog.getByRole("button", { name: "App author", exact: true }).click();
  await page.screenshot({
    path: "test-results/mcp-edit-mobile.png",
    fullPage: true,
  });
  await dialog
    .getByRole("button", { name: "Save access", exact: true })
    .click();
  await expect(dialog).not.toBeVisible();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/console/account");
  await expect(grant).toBeVisible();
  await expect
    .poll(() =>
      page
        .locator(".agent-access")
        .evaluate(
          (panel) =>
            panel.getBoundingClientRect().left -
            document.querySelector(".sidebar")!.getBoundingClientRect().right,
        ),
    )
    .toBeGreaterThan(0);
  await page.evaluate(() => scrollTo(0, 0));
  await page.screenshot({
    path: "test-results/mcp-connections-desktop.png",
    fullPage: true,
    animations: "disabled",
  });
  const rejectRevoke = async (route: import("@playwright/test").Route) => {
    if (route.request().method() === "PATCH") {
      await route.fulfill({
        status: 412,
        json: {
          error: { code: "stale_revision", message: "Refresh before saving." },
        },
      });
    } else {
      await route.continue();
    }
  };
  await page.route("**/v1/agent-access/*", rejectRevoke);
  await grant.getByRole("button", { name: "Revoke", exact: true }).click();
  await confirmDialog(page, "Revoke access");
  await expect(page.getByRole("alert")).toContainText("Refresh before saving.");
  await expect(grant).toBeVisible();
  await expect(
    grant.getByRole("button", { name: "Edit", exact: true }),
  ).toBeEnabled();
  await page.unroute("**/v1/agent-access/*", rejectRevoke);
  await page.getByRole("button", { name: "Refresh connections" }).click();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await grant.getByRole("button", { name: "Revoke", exact: true }).click();
  await confirmDialog(page, "Revoke access");
  await expect(grant).toContainText("revoked");
  await expect(
    grant.getByRole("button", { name: "Edit", exact: true }),
  ).toHaveCount(0);
});

test("agent consent returns after signing in and can be denied", async ({
  page,
}) => {
  await login(page);
  await logout(page);
  const q = new URLSearchParams({
    response_type: "code",
    client_id: "lastapp-development",
    redirect_uri: "http://127.0.0.1:8790/callback",
    resource: apiOrigin + "/mcp",
    code_challenge_method: "S256",
    code_challenge: "a".repeat(43),
    state: "deny-state",
    scope: "read",
  });
  await page.goto("/oauth/authorize?" + q);
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Continue with SSO" }).click();
  await page.locator('input[name=subject][value="operator"]').check();
  await page.getByLabel("Password", { exact: true }).fill("lastapp-local");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Connect an agent" }),
  ).toBeVisible();
  await page.route("http://127.0.0.1:8790/callback**", (route) =>
    route.fulfill({ contentType: "text/html", body: "<h1>Declined</h1>" }),
  );
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Declined" })).toBeVisible();
  expect(new URL(page.url()).searchParams.get("error")).toBe("access_denied");
});
