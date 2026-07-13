"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { input, btn, btnSecondary } from "./ui";
import { CONTACT_TYPES, RANKS } from "./EntityContactsEditor";

// InviteContactButton lets an owner offer a contact responsibility on an
// existing entity to someone who isn't already a known contact. It issues a
// single-use role_claim invite link; when the invitee signs in and accepts, the
// contact is attached to the entity and linked to their account. This is the
// non-admin path for adding a brand-new person ("suggest people in the parent
// object OR an invite link").
export function InviteContactButton({
  entityKind,
  entityName,
}: {
  entityKind: "resource" | "resource_group" | "site" | "facility";
  entityName: string;
}) {
  const [open, setOpen] = useState(false);
  const [contactType, setContactType] = useState(CONTACT_TYPES[0]);
  const [rank, setRank] = useState(RANKS[0]);
  const [busy, setBusy] = useState(false);
  const [url, setUrl] = useState("");
  const [copied, setCopied] = useState(false);
  const [err, setErr] = useState("");

  const generate = async () => {
    setErr("");
    setBusy(true);
    try {
      const res = await api.invites.create({
        kind: "role_claim",
        claim: {
          entity_kind: entityKind,
          entity_id: entityName,
          contact_type: contactType,
          rank,
        },
      });
      setUrl((res as { invite_url: string }).invite_url);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard may be unavailable; the link is still selectable */
    }
  };

  const reset = () => {
    setOpen(false);
    setUrl("");
    setErr("");
  };

  if (!open) {
    return (
      <button
        type="button"
        className="text-xs font-medium text-brand-700 hover:underline"
        onClick={() => setOpen(true)}
      >
        + Invite a contact
      </button>
    );
  }

  return (
    <div className="mt-2 rounded-md border border-gray-200 bg-gray-50 p-3">
      <p className="mb-2 text-xs text-gray-500">
        Generate a single-use link inviting someone to become a contact for this{" "}
        {entityKind.replace("_", " ")}. They accept it after signing in.
      </p>
      {!url ? (
        <>
          <div className="grid gap-2 sm:grid-cols-2">
            <select
              className={input}
              value={contactType}
              onChange={(e) => setContactType(e.target.value)}
            >
              {CONTACT_TYPES.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
            <select className={input} value={rank} onChange={(e) => setRank(e.target.value)}>
              {RANKS.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          </div>
          {err && <p className="mt-2 text-xs text-red-600">{err}</p>}
          <div className="mt-2 flex gap-2">
            <button className={btn} disabled={busy} onClick={generate}>
              {busy ? "Generating…" : "Generate link"}
            </button>
            <button className={btnSecondary} onClick={reset}>
              Cancel
            </button>
          </div>
        </>
      ) : (
        <>
          <div className="flex items-center gap-2">
            <input className={`${input} font-mono text-xs`} readOnly value={url} />
            <button className={btnSecondary} onClick={copy}>
              {copied ? "Copied" : "Copy"}
            </button>
          </div>
          <button className="mt-2 text-xs text-brand-700 hover:underline" onClick={reset}>
            Done
          </button>
        </>
      )}
    </div>
  );
}
