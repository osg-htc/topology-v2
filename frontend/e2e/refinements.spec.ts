import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// A facility can't be submitted without an institution from the registry.
test("facility form requires an institution", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  await page.goto("/facilities/new");
  await page.getByRole("textbox").first().fill(`E2E_Fac_${Date.now()}`); // Name is the first field
  await page.getByRole("button", { name: "Submit for review" }).click();
  // Submission is blocked with an inline error; no navigation to the view page.
  await expect(page.getByText(/Pick an institution/i)).toBeVisible();
  await expect(page).not.toHaveURL(/\/proposals\/view/);
});

// The picker resolves a real institution and then allows submission.
test("facility form accepts a registry institution", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const insts = await (await page.request.get("/api/v1/institutions?q=a")).json();
  test.skip(!insts || insts.length === 0, "needs cached institutions");
  const name: string = insts[0].name;

  await page.goto("/facilities/new");
  await page.getByRole("textbox").first().fill(`E2E_Fac_${Date.now()}`);
  await page.getByPlaceholder("Search your institution by name…").fill(name);
  await page.getByRole("button", { name }).first().click();
  await expect(page.getByText(/Institution id:/)).toBeVisible();
});

// The change-request list summarizes what each request changes.
test("proposal list shows a kind chip and change summary", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");

  const rgs = await (await page.request.get("/api/v1/resource-groups")).json();
  test.skip(!rgs || rgs.length === 0, "needs a resource group");
  const rg: string = rgs[0].name;
  const name = `E2E_Sum_${Date.now()}`;
  await page.request.post("/api/v1/proposals", {
    data: {
      entity_kind: "resource",
      operation: "create",
      submit: true,
      proposed_state: { name, resource_group: rg, resource: { Active: true, FQDN: `${name.toLowerCase()}.example.org` } },
    },
  });

  await page.goto("/proposals");
  await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
  const row = page.getByRole("link").filter({ hasText: name });
  await expect(row.getByText("Resource", { exact: true })).toBeVisible();
  await expect(row.getByText(/host .*example\.org/)).toBeVisible();
});
