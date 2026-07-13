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
  await page.getByPlaceholder("e.g. UChicago_OSGConnect_ap20").fill(unique);
  await page.getByPlaceholder("Search resource groups…").fill(rgName);
  // Host names must be valid DNS names (no underscores).
  await page.getByPlaceholder("host.example.org").fill(`${unique.toLowerCase().replace(/_/g, "-")}.example.org`);
  // A contact is required for a complete registration. As an admin the person
  // field is a live user search ("Search people…"); external ids are no longer
  // shown — picking a name is enough.
  await page.getByPlaceholder("Search people…").first().fill("E2E Admin");

  const submit = page.getByRole("button", { name: "Submit for review" });
  await expect(submit).toBeEnabled();
  await submit.click();

  await expect(page).toHaveURL(/\/proposals\/view/);
  await expect(page.getByText("pending")).toBeVisible();
  // The structured proposed-change view shows the resource name exactly (the
  // FQDN also contains it, so match exactly).
  await expect(page.getByText(unique, { exact: true })).toBeVisible();
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
