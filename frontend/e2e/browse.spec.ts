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
