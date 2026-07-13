import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// A user proposes themselves for a contact slot; the incumbent approves the
// hand-off from the "Contact hand-offs" page and the slot is repointed.
test("propose-self replacement is approved by the incumbent", async ({ page, browser }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const ts = Date.now();
  const fac = `RepFac_${ts}`;
  const site = `RepSite_${ts}`;
  const rg = `RepRG_${ts}`;

  // Create a fresh resource group via an atomic bundle so the slot starts clean.
  const bundle = await page.request.post("/api/v1/proposals", {
    data: {
      entity_kind: "bundle",
      operation: "create",
      submit: true,
      proposed_state: {
        operations: [
          { entity_kind: "facility", operation: "create", proposed_state: { name: fac, institution_id: "" } },
          { entity_kind: "site", operation: "create", proposed_state: { name: site, facility: fac, long_name: site } },
          { entity_kind: "resource_group", operation: "create", proposed_state: { name: rg, site } },
        ],
      },
    },
  });
  const pid = (await bundle.json()).id;
  await page.request.post(`/api/v1/proposals/${pid}/approve`);

  // The incumbent claims Admin/Primary via a role_claim invite.
  const inv = await page.request.post("/api/v1/invites", {
    data: { kind: "role_claim", claim: { entity_kind: "resource_group", entity_id: rg, contact_type: "Administrative Contact", rank: "Primary" } },
  });
  const claimToken = (await inv.json()).token;

  const incCtx = await browser.newContext();
  const inc = await incCtx.newPage();
  await devLogin(inc, "user", `incumbent_${ts}@example.org`);
  await inc.request.post(`/api/v1/invites/${claimToken}/accept`);

  // The requester opens the RG detail page and proposes themselves for the slot.
  const reqCtx = await browser.newContext();
  const req = await reqCtx.newPage();
  await devLogin(req, "user", `requester_${ts}@example.org`);
  await req.goto(`/resource-groups/detail?name=${encodeURIComponent(rg)}`);
  await req.getByRole("button", { name: "propose myself" }).click();
  await expect(req.getByText("Request sent")).toBeVisible();

  // The incumbent approves it from the hand-offs page.
  await inc.goto("/replacements");
  await expect(inc.getByText(/wants to take over/)).toBeVisible();
  await inc.getByRole("button", { name: "Approve" }).click();

  // The requester's request is now approved.
  await expect(async () => {
    const mine = await (await req.request.get("/api/v1/contact-replacements/mine")).json();
    expect(mine[0]?.status).toBe("approved");
  }).toPass();

  await incCtx.close();
  await reqCtx.close();
});
