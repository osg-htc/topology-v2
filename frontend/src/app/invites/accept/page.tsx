"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import { Card, btn } from "@/components/ui";

function AcceptInner() {
  const params = useSearchParams();
  const router = useRouter();
  const token = params.get("token") || "";
  const [msg, setMsg] = useState("");

  const { data: invite } = useQuery({
    queryKey: ["invite", token],
    queryFn: () => api.invites.get(token),
    enabled: !!token,
    retry: false,
  });
  const { data: session, isLoading: sessionLoading } = useQuery({
    queryKey: ["me"],
    queryFn: api.auth.me,
    retry: false,
  });
  const needsLogin = !sessionLoading && !session;

  const signInToAccept = () => {
    window.location.href = `/api/v1/auth/oidc/login?invite=${encodeURIComponent(token)}`;
  };

  const accept = async () => {
    setMsg("");
    try {
      await api.invites.accept(token);
      router.replace("/");
    } catch (e) {
      setMsg(e instanceof ApiError ? e.message : String(e));
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-navy-950 p-4">
      <Card className="w-full max-w-md">
        <h1 className="mb-2 text-xl font-bold text-navy-900">Accept invitation</h1>
        {!token ? (
          <p className="text-sm text-red-600">Missing invite token.</p>
        ) : invite && !invite.valid ? (
          <p className="text-sm text-red-600">This invite has expired or was already used.</p>
        ) : (
          <>
            {invite?.claim && (
              <div className="mb-4 rounded border border-gray-200 bg-gray-50 p-3 text-sm">
                You are invited to become the{" "}
                <span className="font-medium">
                  {invite.claim.rank} {invite.claim.contact_type}
                </span>{" "}
                on <span className="font-medium">{invite.claim.entity_id}</span>.
              </div>
            )}
            {needsLogin ? (
              <button className={`${btn} w-full justify-center`} onClick={signInToAccept}>
                Sign in with CILogon to accept
              </button>
            ) : (
              <button className={`${btn} w-full justify-center`} onClick={accept}>
                Accept responsibility
              </button>
            )}
            {msg && <p className="mt-3 text-sm text-red-600">{msg}</p>}
          </>
        )}
      </Card>
    </div>
  );
}

export default function AcceptInvitePage() {
  return (
    <Suspense fallback={null}>
      <AcceptInner />
    </Suspense>
  );
}
