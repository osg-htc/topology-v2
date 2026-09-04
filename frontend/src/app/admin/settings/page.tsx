"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { PageHeader, Card, btn, input, label } from "@/components/ui";

export default function AdminSettingsPage() {
  const qc = useQueryClient();
  const { data, isLoading, error } = useQuery({
    queryKey: ["oidc-config"],
    queryFn: api.admin.getOIDCConfig,
    retry: false,
  });
  const [issuer, setIssuer] = useState("");
  const [clientID, setClientID] = useState("");
  const [secret, setSecret] = useState("");
  const [msg, setMsg] = useState("");

  useEffect(() => {
    if (data) {
      setIssuer(data.issuer);
      setClientID(data.client_id);
    }
  }, [data]);

  const save = useMutation({
    mutationFn: () =>
      api.admin.setOIDCConfig({
        issuer,
        client_id: clientID,
        ...(secret ? { client_secret: secret } : {}),
      }),
    onSuccess: () => {
      setMsg("Saved. New logins will use the updated settings.");
      setSecret("");
      qc.invalidateQueries({ queryKey: ["oidc-config"] });
    },
    onError: (e) => setMsg(String(e)),
  });

  if (error) {
    // Only a real 403 means "you're not an admin" -- any other failure must
    // show its own message, not blame the viewer's role.
    const forbidden = error instanceof ApiError && error.status === 403;
    return (
      <div className="p-8">
        <PageHeader title="Settings" />
        <Card>
          <p className="text-sm text-red-600">
            {forbidden ? "Administrator role required." : String(error)}
          </p>
        </Card>
      </div>
    );
  }

  return (
    <div className="p-8">
      <PageHeader title="Settings" />
      <Card className="max-w-xl">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-gray-500">
          OIDC / CILogon
        </h2>
        {isLoading ? (
          <p className="text-gray-400">Loading…</p>
        ) : (
          <div className="space-y-4">
            <div>
              <label className={label}>Issuer</label>
              <input className={input} value={issuer} onChange={(e) => setIssuer(e.target.value)} />
            </div>
            <div>
              <label className={label}>Client ID</label>
              <input className={input} value={clientID} onChange={(e) => setClientID(e.target.value)} />
            </div>
            <div>
              <label className={label}>Client secret</label>
              <input
                className={input}
                type="password"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder={data?.has_secret ? "•••••• (leave blank to keep)" : "not set"}
              />
              <p className="mt-1 text-xs text-gray-400">
                Stored encrypted. Leave blank to keep the existing secret.
              </p>
            </div>
            {msg && <p className="text-sm text-gray-600">{msg}</p>}
            <button className={btn} disabled={save.isPending} onClick={() => save.mutate()}>
              Save OIDC settings
            </button>
          </div>
        )}
      </Card>
    </div>
  );
}
