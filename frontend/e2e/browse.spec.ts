import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// Navigating to Resources must render the table without a runtime error — this
// regression-guards the /api/v1/resources JSON shape the page depends on
// (a key-case mismatch previously crashed the filter on r.name.toLowerCase()).
test("resources page renders without crashing", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await devLogin(page, "administrator", "admin@example.org");
  await page.getByRole("link", { name: "Resources", exact: true }).click();

  await expect(page).toHaveURL(/\/resources/);
  await expect(page.getByRole("heading", { name: "Resources" })).toBeVisible();
  // The search box (and thus the filter that crashed) is present and usable.
  await page.getByPlaceholder(/Search by name/).fill("x");

  expect(errors, `uncaught page errors: ${errors.join("; ")}`).toHaveLength(0);
});

// Clicking a resource opens its detail page with the structured sections.
test("resource detail page opens with contacts and services sections", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const resources = await (await page.request.get("/api/v1/resources")).json();
  const names = Object.keys(resources);
  test.skip(names.length === 0, "needs at least one resource");
  await page.goto(`/resources/detail?name=${encodeURIComponent(names[0])}`);
  await expect(page.getByRole("heading", { name: "Services" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Contacts" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Placement" })).toBeVisible();
});
