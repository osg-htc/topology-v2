import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// Exercises the register-a-resource -> proposal flow end to end through the UI.
test("register a resource creates a proposal", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");

  await page.getByRole("link", { name: "Register a resource" }).first().click();
  await expect(page).toHaveURL(/\/proposals\/new/);

  const unique = `E2E_Res_${Date.now()}`;
  await page.getByText("Resource name", { exact: false }); // ensure form rendered
  const inputs = page.locator("input");
  await inputs.nth(0).fill(unique); // resource name
  await inputs.nth(1).fill("E2E_RG"); // resource group
  await inputs.nth(2).fill(`${unique.toLowerCase()}.example.org`); // FQDN

  await page.getByRole("button", { name: "Submit for review" }).click();

  // Lands on the proposal detail with a pending status and the editable state.
  await expect(page).toHaveURL(/\/proposals\/view/);
  await expect(page.getByText("pending")).toBeVisible();
  await expect(page.getByText(unique)).toBeVisible();
});

test("schema validation blocks an invalid resource (no FQDN)", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  // page.request shares the browser context's session cookie.
  const res = await page.request.post("/api/v1/proposals", {
    data: {
      entity_kind: "resource",
      operation: "create",
      proposed_state: { name: "X", resource_group: "RG", resource: { Active: true } },
    },
  });
  expect(res.status()).toBe(400);
  expect(await res.text()).toContain("FQDN");
});
