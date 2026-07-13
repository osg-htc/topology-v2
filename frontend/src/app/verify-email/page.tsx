"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn } from "@/components/ui";

function VerifyEmailView() {
  const token = useSearchParams().get("token") || "";
  const [state, setState] = useState<"working" | "done" | "error">("working");
  const [msg, setMsg] = useState("");
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return; // confirm exactly once
    ran.current = true;
    if (!token) {
      setState("error");
      setMsg("This link is missing its token.");
      return;
    }
    api.emailVerifications
      .confirm(token)
      .then((r) => {
        setState("done");
        setMsg(r.email);
      })
      .catch((e) => {
        setState("error");
        setMsg(String(e).replace(/^Error:\s*/, ""));
      });
  }, [token]);

  return (
    <div className="p-8">
      <PageHeader title="Verify email" />
      <Card className="max-w-lg">
        {state === "working" && <p className="text-sm text-gray-500">Verifying…</p>}
        {state === "done" && (
          <>
            <p className="text-sm text-gray-800">
              <span className="mr-1 rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-800">verified</span>
              {msg} is now confirmed.
            </p>
            <Link href="/account" className={`${btn} mt-4 inline-block`}>
              Back to account
            </Link>
          </>
        )}
        {state === "error" && (
          <>
            <p className="text-sm text-red-600">{msg}</p>
            <Link href="/account" className={`${btn} mt-4 inline-block`}>
              Back to account
            </Link>
          </>
        )}
      </Card>
    </div>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense fallback={null}>
      <VerifyEmailView />
    </Suspense>
  );
}
