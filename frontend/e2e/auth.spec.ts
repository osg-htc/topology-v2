import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

test("unauthenticated users are redirected to login", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
  await expect(page.getByRole("button", { name: /CILogon/ })).toBeVisible();
});

test("dev login lands on the dashboard with the sidebar", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  await expect(page.getByText("OSG Topology")).toBeVisible();
  // Admin sees the admin nav.
  await expect(page.getByRole("link", { name: "Backup & restore" })).toBeVisible();
});

test("a plain user does not see admin navigation", async ({ page }) => {
  await devLogin(page, "user", "user@example.org");
  await expect(page.getByRole("link", { name: "Audit log" })).toHaveCount(0);
});
