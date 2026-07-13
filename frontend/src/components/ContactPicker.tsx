"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { input } from "./ui";
import { ContactPersonInput } from "./ContactPersonInput";

// The subset of a contact row this control manages.
export type ContactValue = {
  name: string;
  id: string;
  inviteId?: string;
  invitePending?: boolean;
  inviteUrl?: string;
};

// ContactPicker selects a contact person WITHOUT exposing external ids: either
// pick an existing user (search), or onboard a brand-new person via an invite
// link. When invited, the row is marked pending — the change can't be committed
// until the invite is accepted.
export function ContactPicker({
  value,
  isAdmin,
  fallback,
  onChange,
}: {
  value: ContactValue;
  isAdmin: boolean;
  fallback: { name: string; id: string }[];
  onChange: (patch: Partial<ContactValue>) => void;
}) {
  const [mode, setMode] = useState<"pick" | "invite">("pick");
  const [inviteName, setInviteName] = useState("");
  const [inviteEmail, setInviteEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  // Already-invited: show a pending chip and the copyable link.
  if (value.invitePending) {
    return (
      <div className="rounded-md border border-amber-200 bg-amber-50 p-2 text-xs">
        <div className="mb-1 flex items-center gap-2">
          <span className="rounded-full bg-amber-200 px-2 py-0.5 text-amber-900">invite pending</span>
          <span className="text-gray-700">{value.name}</span>
          <button
            type="button"
            className="ml-auto text-brand-700 hover:underline"
            onClick={() => onChange({ name: "", id: "", inviteId: undefined, invitePending: false, inviteUrl: undefined })}
          >
            clear
          </button>
        </div>
        {value.inviteUrl && (
          <input
            className="w-full rounded border border-amber-200 bg-white px-1 py-0.5 font-mono text-[10px]"
            readOnly
            value={value.inviteUrl}
            onFocus={(e) => e.currentTarget.select()}
          />
        )}
        <p className="mt-1 text-gray-500">Send this link to {value.name}. The change can be submitted now but only approved once they accept.</p>
      </div>
    );
  }

  const generate = async () => {
    setErr("");
    if (!inviteName.trim()) {
      setErr("Name is required.");
      return;
    }
    setBusy(true);
    try {
      const res = (await api.invites.create({
        kind: "contact_onboard",
        display_name: inviteName.trim(),
        email: inviteEmail.trim() || undefined,
      })) as unknown as { invite_url: string; invite_id: string; name?: string };
      onChange({
        name: res.name ?? inviteName.trim(),
        id: "",
        inviteId: res.invite_id,
        invitePending: true,
        inviteUrl: res.invite_url,
      });
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  };

  if (mode === "invite") {
    return (
      <div className="rounded-md border border-gray-200 bg-gray-50 p-2">
        <div className="grid grid-cols-2 gap-2">
          <input className={input} value={inviteName} onChange={(e) => setInviteName(e.target.value)} placeholder="Full name" />
          <input className={input} type="email" value={inviteEmail} onChange={(e) => setInviteEmail(e.target.value)} placeholder="Email (optional)" />
        </div>
        {err && <p className="mt-1 text-xs text-red-600">{err}</p>}
        <div className="mt-2 flex gap-3 text-xs">
          <button type="button" className="text-brand-700 hover:underline disabled:opacity-40" disabled={busy} onClick={generate}>
            Generate invite link
          </button>
          <button type="button" className="text-gray-500 hover:underline" onClick={() => setMode("pick")}>
            Cancel
          </button>
        </div>
      </div>
    );
  }

  return (
    <div>
      <ContactPersonInput
        name={value.name}
        id={value.id}
        isAdmin={isAdmin}
        fallback={fallback}
        onChange={(nm, cid) => onChange({ name: nm, id: cid })}
      />
      <button type="button" className="mt-1 text-xs text-brand-600 hover:underline" onClick={() => setMode("invite")}>
        …or invite someone new
      </button>
    </div>
  );
}
