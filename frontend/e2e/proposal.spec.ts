import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// Exercises the register-a-resource -> proposal flow end to end through the UI.
// The RG field is a dropdown that requires an existing resource group, so the
// test picks a real one from the API.
test("register a resource creates a proposal", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");

  const rgs = await (await page.request.get("/api/v1/resource-groups")).json();
  test.skip(!rgs || rgs.length === 0, "needs at least one resource group (import data first)");
  const rgName: string = rgs[0].name;

  await page.goto("/proposals/new");
  const unique = `E2E_Res_${Date.now()}`;
  const inputs = page.locator("input");
  await inputs.nth(0).fill(unique); // resource name
  await inputs.nth(1).fill(rgName); // resource group (must be an existing RG)
  await inputs.nth(2).fill(`${unique.toLowerCase()}.example.org`); // FQDN

  const submit = page.getByRole("button", { name: "Submit for review" });
  await expect(submit).toBeEnabled();
  await submit.click();

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
