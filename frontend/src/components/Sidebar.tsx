"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { SessionInfo } from "@/lib/api";

const nav = [
  { href: "/", label: "Dashboard" },
  { href: "/resources", label: "Resources" },
  { href: "/resource-groups", label: "Resource groups" },
  { href: "/sites", label: "Sites" },
  { href: "/facilities", label: "Facilities" },
  { href: "/projects", label: "Projects" },
  { href: "/downtimes", label: "Downtimes" },
  { href: "/proposals", label: "My requests" },
  { href: "/replacements", label: "Contact hand-offs" },
];

// Not manager/admin-gated: a contact of a resource can decide pending
// downtime proposals for it (see canDecideProposal on the backend), so
// everyone gets this link -- the page itself just renders whatever the API
// returns, empty state included, same as My requests/Contact hand-offs.
const reviewerNav = [{ href: "/proposals/review", label: "Review queue" }];
const adminNav = [
  { href: "/admin/users", label: "Users" },
  { href: "/admin/settings", label: "Settings" },
  { href: "/institutions", label: "Institutions" },
  { href: "/admin/audit", label: "Audit log" },
  { href: "/admin/backup", label: "Backup & restore" },
];

export function Sidebar({ session }: { session: SessionInfo }) {
  const pathname = usePathname();
  const role = session.effective_role;
  const isAdmin = role === "administrator";

  const link = (href: string, label: string) => {
    const active = pathname === href;
    return (
      <Link
        key={href}
        href={href}
        className={`block rounded px-3 py-2 text-sm ${
          active ? "bg-brand-600 text-white" : "text-navy-100 hover:bg-navy-800"
        }`}
      >
        {label}
      </Link>
    );
  };

  return (
    <aside className="flex w-64 flex-col bg-navy-900 text-white">
      <div className="px-4 py-5 text-lg font-semibold">OSG Topology</div>
      <nav className="flex-1 space-y-1 px-2">
        {nav.map((n) => link(n.href, n.label))}
        <div className="px-3 pt-4 text-xs uppercase tracking-wide text-navy-100/60">
          Review
        </div>
        {reviewerNav.map((n) => link(n.href, n.label))}
        {isAdmin && (
          <>
            <div className="px-3 pt-4 text-xs uppercase tracking-wide text-navy-100/60">
              Admin
            </div>
            {adminNav.map((n) => link(n.href, n.label))}
          </>
        )}
      </nav>
      <Link
        href="/account"
        className="block border-t border-navy-800 px-4 py-3 text-xs text-navy-100/80 hover:bg-navy-800"
      >
        <div className="font-medium text-white">{session.user.display_name}</div>
        <div className="capitalize">{role} · account settings</div>
      </Link>
    </aside>
  );
}
