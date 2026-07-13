"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, input, label } from "@/components/ui";

export default function AccountPage() {
  const qc = useQueryClient();
  const { data: me } = useQuery({ queryKey: ["me"], queryFn: api.auth.me, retry: false });
  const { data: emails } = useQuery({ queryKey: ["email-verifications"], queryFn: api.emailVerifications.list });

  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [devLink, setDevLink] = useState("");

  const request = async () => {
    setErr("");
    setDevLink("");
    setBusy(true);
    try {
      const res = await api.emailVerifications.request(email.trim());
      setEmail("");
      qc.invalidateQueries({ queryKey: ["email-verifications"] });
      // In development the backend returns the link directly (no SMTP).
      if (res.verify_url) setDevLink(res.verify_url);
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="p-8">
      <PageHeader title="Account" description="Your profile and verified email addresses." />

      <Card className="mb-6 max-w-xl">
        <h3 className="mb-3 text-sm font-semibold text-gray-700">Profile</h3>
        <dl className="grid grid-cols-2 gap-y-2 text-sm">
          <dt className="text-gray-400">Display name</dt>
          <dd className="text-gray-800">{me?.user.display_name}</dd>
          <dt className="text-gray-400">Role</dt>
          <dd className="capitalize text-gray-800">{me?.effective_role}</dd>
          <dt className="text-gray-400">User ID</dt>
          <dd className="font-mono text-xs text-gray-500">{me?.user.id}</dd>
        </dl>
      </Card>

      <Card className="max-w-xl">
        <h3 className="mb-3 text-sm font-semibold text-gray-700">Email addresses</h3>
        {(emails ?? []).length === 0 ? (
          <p className="mb-4 text-sm text-gray-400">No emails on file yet.</p>
        ) : (
          <ul className="mb-4 space-y-1 text-sm">
            {(emails ?? []).map((e, i) => (
              <li key={i} className="flex items-center justify-between border-b border-gray-100 py-1">
                <span className="text-gray-700">{e.email_hint}</span>
                <span
                  className={`rounded-full px-2 py-0.5 text-xs ${
                    e.verified ? "bg-green-100 text-green-800" : "bg-amber-100 text-amber-800"
                  }`}
                >
                  {e.verified ? "verified" : "pending"}
                </span>
              </li>
            ))}
          </ul>
        )}

        <label className={label}>Add and verify an email</label>
        <div className="flex gap-2">
          <input
            className={input}
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.org"
          />
          <button className={btn} disabled={busy || !email.trim()} onClick={request}>
            Send link
          </button>
        </div>
        {err && <p className="mt-2 text-sm text-red-600">{err}</p>}
        {devLink && (
          <p className="mt-2 break-all text-xs text-gray-500">
            Dev mode — open this link to verify:{" "}
            <a href={devLink} className="text-brand-700 hover:underline">
              {devLink}
            </a>
          </p>
        )}
        <p className="mt-3 text-xs text-gray-400">
          We send a single-use link to the address; the address is confirmed once you open it. Your
          email is stored encrypted — only a masked hint is shown here.
        </p>
      </Card>
    </div>
  );
}
