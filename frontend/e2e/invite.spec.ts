import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// An owner can generate a role_claim invite for a resource group from its detail
// page; a different user accepting it becomes a contact on that group.
test("owner invites a contact and the invitee is attached on accept", async ({ page, browser }) => {
  await devLogin(page, "user", "owner@example.org");

  const rgs = await (await page.request.get("/api/v1/resource-groups")).json();
  test.skip(!rgs || rgs.length === 0, "needs a resource group (import data first)");
  const rg: string = rgs[0].name;

  await page.goto(`/resource-groups/detail?name=${encodeURIComponent(rg)}`);
  await page.getByRole("button", { name: "+ Invite a contact" }).click();
  await page.getByRole("button", { name: "Generate link" }).click();

  const linkField = page.locator('input[readonly]');
  await expect(linkField).toBeVisible();
  const url = await linkField.inputValue();
  const token = new URL(url).searchParams.get("token")!;
  expect(token).toBeTruthy();

  // A second user accepts the invite in a fresh context.
  const ctx = await browser.newContext();
  const p2 = await ctx.newPage();
  await devLogin(p2, "user", `invitee_${Date.now()}@example.org`);
  const accept = await p2.request.post(`/api/v1/invites/${token}/accept`);
  expect(accept.ok()).toBeTruthy();
  await ctx.close();

  // The invitee now shows up as a contact on the resource group.
  const detail = await (await page.request.get(`/api/v1/resource-groups/${encodeURIComponent(rg)}`)).json();
  expect((detail.contacts ?? []).length).toBeGreaterThan(0);
});
