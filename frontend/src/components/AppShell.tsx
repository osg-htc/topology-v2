"use client";

import { useQuery } from "@tanstack/react-query";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { api, ApiError } from "@/lib/api";
import { Sidebar } from "./Sidebar";

// Routes that render without the authenticated shell.
const PUBLIC_PREFIXES = ["/login", "/invites/accept"];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const isPublic = PUBLIC_PREFIXES.some((p) => pathname.startsWith(p));

  const { data: session, isLoading, error } = useQuery({
    queryKey: ["me"],
    queryFn: api.auth.me,
    retry: false,
    enabled: !isPublic, // don't probe auth on public routes (login, invite accept)
  });

  useEffect(() => {
    if (!isPublic && error instanceof ApiError && error.status === 401) {
      router.replace(`/login?return_to=${encodeURIComponent(pathname)}`);
    }
  }, [error, isPublic, pathname, router]);

  if (isPublic) return <>{children}</>;
  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center text-gray-400">
        Loading…
      </div>
    );
  }
  if (!session) return null;

  return (
    <div className="flex h-screen">
      <Sidebar session={session} />
      <main className="flex-1 overflow-auto bg-gray-50">{children}</main>
    </div>
  );
}
