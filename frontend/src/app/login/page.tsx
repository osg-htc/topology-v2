"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { btn, input, label, Card } from "@/components/ui";

// account_link/contact_onboard invites are redeemed by signing in through
// this page (not /invites/accept -- see CreateInvite's acceptPath), and
// carry no `claim` block (that's role_claim/replacement_request only), so
// the invite-context banner below is deliberately just a one-line label per
// kind plus the contact_email it was addressed to, when there is one.
const inviteKindLabel: Record<string, string> = {
  account_link: "link this invite to your account",
  contact_onboard: "accept this contact invitation",
};

function LoginInner() {
  const params = useSearchParams();
  const router = useRouter();
  const returnTo = params.get("return_to") || "/";
  const invite = params.get("invite") || "";
  const { data: mode } = useQuery({ queryKey: ["authmode"], queryFn: api.auth.mode });
  const { data: inviteInfo } = useQuery({
    queryKey: ["invite", invite],
    queryFn: () => api.invites.get(invite),
    enabled: !!invite,
    retry: false,
  });
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("administrator");
  const [err, setErr] = useState("");

  const oidcLogin = () => {
    const q = new URLSearchParams({ return_to: returnTo });
    if (invite) q.set("invite", invite);
    window.location.href = `/api/v1/auth/oidc/login?${q.toString()}`;
  };

  const devLogin = async () => {
    setErr("");
    try {
      await api.auth.devLogin({ email: email || undefined, role, invite: invite || undefined });
      router.replace(returnTo);
    } catch (e) {
      setErr(String(e));
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-navy-950 p-4">
      <Card className="w-full max-w-sm">
        <h1 className="mb-1 text-xl font-bold text-navy-900">OSG Topology</h1>
        <p className="mb-6 text-sm text-gray-500">Sign in to continue</p>

        {invite && inviteInfo && (
          <div
            className={`mb-4 rounded border p-3 text-sm ${
              inviteInfo.valid
                ? "border-gray-200 bg-gray-50 text-gray-700"
                : "border-red-200 bg-red-50 text-red-700"
            }`}
          >
            {inviteInfo.valid ? (
              <>
                Sign in to {inviteKindLabel[inviteInfo.kind] ?? "accept this invite"}.
                {inviteInfo.contact_email && (
                  <>
                    {" "}
                    Sent to <span className="font-medium">{inviteInfo.contact_email}</span>.
                  </>
                )}
              </>
            ) : (
              "This invite has expired or was already used — signing in won't redeem it."
            )}
          </div>
        )}

        <button className={`${btn} w-full justify-center`} onClick={oidcLogin}>
          Sign in with CILogon
        </button>

        {mode?.mode === "dev" && (
          <div className="mt-6 border-t border-gray-200 pt-4">
            <p className="mb-3 text-xs font-semibold uppercase text-gray-400">
              Dev login
            </p>
            <label className={label}>Email</label>
            <input
              className={input}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.org"
            />
            <label className={`${label} mt-3`}>Role</label>
            <select className={input} value={role} onChange={(e) => setRole(e.target.value)}>
              <option value="administrator">administrator</option>
              <option value="manager">manager</option>
              <option value="user">user</option>
            </select>
            <button className={`${btn} mt-4 w-full justify-center`} onClick={devLogin}>
              Dev sign in
            </button>
            {err && <p className="mt-2 text-xs text-red-600">{err}</p>}
          </div>
        )}
      </Card>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <LoginInner />
    </Suspense>
  );
}
