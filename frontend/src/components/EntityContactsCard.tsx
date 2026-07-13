"use client";

import { useQuery } from "@tanstack/react-query";
import { api, EntityContact } from "@/lib/api";
import { Card } from "./ui";
import { InviteContactButton } from "./InviteContactButton";
import { ContactReplaceActions } from "./ContactReplaceActions";

// EntityContactsCard renders the contacts registered on a resource group / site
// / facility, with an owner-only "Invite a contact" action.
export function EntityContactsCard({
  entityKind,
  entityName,
  contacts,
}: {
  entityKind: "resource_group" | "site" | "facility";
  entityName: string;
  contacts?: EntityContact[];
}) {
  const { data: session } = useQuery({ queryKey: ["me"], queryFn: api.auth.me, retry: false });
  const canManage = !!session?.user?.id;
  const rows = contacts ?? [];

  return (
    <Card className="mt-6">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-700">Contacts</h3>
        {canManage && <InviteContactButton entityKind={entityKind} entityName={entityName} />}
      </div>
      {rows.length === 0 ? (
        <p className="text-sm text-gray-400">
          No contacts at this level. Resources inherit contacts from their enclosing scopes.
        </p>
      ) : (
        <table className="min-w-full text-sm">
          <thead className="text-left text-xs uppercase tracking-wide text-gray-400">
            <tr>
              <th className="py-1 pr-4">Type</th>
              <th className="py-1 pr-4">Rank</th>
              <th className="py-1 pr-4">Name</th>
              <th className="py-1 pr-4">ID</th>
              {canManage && <th className="py-1">Take over</th>}
            </tr>
          </thead>
          <tbody>
            {rows.map((c, i) => (
              <tr key={i} className="border-t border-gray-100">
                <td className="py-1 pr-4 text-gray-700">{c.contact_type}</td>
                <td className="py-1 pr-4 text-gray-500">{c.rank}</td>
                <td className="py-1 pr-4 text-gray-700">{c.name || "—"}</td>
                <td className="py-1 pr-4 text-xs text-gray-400">{c.id || "—"}</td>
                {canManage && (
                  <td className="py-1">
                    <ContactReplaceActions
                      entityKind={entityKind}
                      entityName={entityName}
                      contactType={c.contact_type}
                      rank={c.rank}
                    />
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Card>
  );
}
