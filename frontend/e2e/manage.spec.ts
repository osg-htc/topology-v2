import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// Browsing the management pages must render without runtime errors, and the
// dashboard must show the all-topology summary tiles.
test("dashboard shows summary tiles", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  await expect(page.getByText("All topology")).toBeVisible();
  // The summary section links each entity type; scope to it to avoid matching
  // the sidebar nav of the same name.
  const section = page.locator("section", { hasText: "All topology" });
  await expect(section.getByRole("link", { name: /Resources$/ })).toBeVisible();
});

for (const { label, heading } of [
  { label: "Resource groups", heading: "Resource groups" },
  { label: "Sites", heading: "Sites" },
  { label: "Facilities", heading: "Facilities" },
  { label: "Institutions", heading: "Institutions" },
]) {
  test(`${label} page renders without crashing`, async ({ page }) => {
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    await devLogin(page, "administrator", "admin@example.org");
    await page.getByRole("link", { name: label, exact: true }).click();
    await expect(page.getByRole("heading", { name: heading })).toBeVisible();
    expect(errors, errors.join("; ")).toHaveLength(0);
  });
}

test("resource form uses an RG dropdown with a create-new link", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  await page.goto("/proposals/new");
  // RG field is backed by a datalist (dropdown/search), not free text validation.
  await expect(page.locator("#rg-options")).toHaveCount(1);
  await expect(page.getByRole("link", { name: /New resource group/ })).toBeVisible();
});

test("admin settings exposes OIDC configuration", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  await page.getByRole("link", { name: "Settings", exact: true }).click();
  await expect(page.getByText("OIDC / CILogon")).toBeVisible();
});
