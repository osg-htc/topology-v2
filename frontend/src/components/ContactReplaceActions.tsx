"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

// ContactReplaceActions offers, for one contact slot, two ways to take it over:
// propose yourself (files a replacement request the incumbent or a manager
// approves), or generate an invite link that lets someone else propose
// themselves. Rendered only for signed-in users.
export function ContactReplaceActions({
  entityKind,
  entityName,
  contactType,
  rank,
}: {
  entityKind: string;
  entityName: string;
  contactType: string;
  rank: string;
}) {
  const { data: session } = useQuery({ queryKey: ["me"], queryFn: api.auth.me, retry: false });
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");
  const [url, setUrl] = useState("");

  if (!session?.user?.id) return null;

  const proposeSelf = async () => {
    setBusy(true);
    setMsg("");
    try {
      await api.contactReplacements.create({ entity_kind: entityKind, entity_name: entityName, contact_type: contactType, rank });
      setMsg("Request sent");
    } catch (e) {
      setMsg(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  };

  const invite = async () => {
    setBusy(true);
    setMsg("");
    try {
      const res = await api.invites.create({
        kind: "replacement_request",
        claim: { entity_kind: entityKind, entity_id: entityName, contact_type: contactType, rank },
      });
      setUrl((res as { invite_url: string }).invite_url);
    } catch (e) {
      setMsg(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  };

  if (url) {
    return (
      <span className="inline-flex items-center gap-1 text-xs">
        <input
          className="w-48 rounded border border-gray-200 px-1 py-0.5 font-mono text-[10px]"
          readOnly
          value={url}
          onFocus={(e) => e.currentTarget.select()}
        />
        <button className="text-brand-700 hover:underline" onClick={() => setUrl("")}>done</button>
      </span>
    );
  }

  return (
    <span className="inline-flex items-center gap-2 text-xs">
      <button className="text-brand-700 hover:underline disabled:opacity-40" disabled={busy} onClick={proposeSelf}>
        propose myself
      </button>
      <span className="text-gray-300">·</span>
      <button className="text-brand-700 hover:underline disabled:opacity-40" disabled={busy} onClick={invite}>
        invite
      </button>
      {msg && <span className="text-gray-500">{msg}</span>}
    </span>
  );
}
