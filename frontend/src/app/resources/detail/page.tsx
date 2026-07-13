"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card } from "@/components/ui";
import { DetailField } from "@/components/DetailField";
import { DowntimesTable } from "@/components/DowntimesTable";
import { InviteContactButton } from "@/components/InviteContactButton";
import { ContactReplaceActions } from "@/components/ContactReplaceActions";

function ResourceDetailView() {
  const name = useSearchParams().get("name") || "";
  const { data: r, isLoading } = useQuery({
    queryKey: ["resource-detail", name],
    queryFn: () => api.resourceDetail(name),
    enabled: !!name,
  });
  const { data: session } = useQuery({ queryKey: ["me"], queryFn: api.auth.me, retry: false });
  const canManage = !!session?.user?.id;

  if (isLoading) return <div className="p-8 text-gray-400">Loading…</div>;
  if (!r) return <div className="p-8 text-gray-400">Resource not found.</div>;

  return (
    <div className="p-8">
      <PageHeader
        title={r.name}
        description="Resource"
        action={
          <span
            className={`rounded-full px-3 py-1 text-xs font-medium ${
              r.active ? "bg-brand-100 text-brand-800" : "bg-gray-100 text-gray-500"
            }`}
          >
            {r.active ? "active" : "inactive"}
          </span>
        }
      />

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <Card>
            <dl className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
              <DetailField label="FQDN" value={r.fqdn} />
              <DetailField label="ID" value={r.id} />
              <DetailField label="DN" value={r.dn} />
              <DetailField label="FQDN aliases" value={(r.fqdn_aliases ?? []).join(", ")} />
              <DetailField label="Tags" value={(r.tags ?? []).join(", ")} />
              <DetailField label="Allowed VOs" value={(r.allowed_vos ?? []).join(", ")} />
            </dl>
            {r.description && (
              <div className="mt-4">
                <div className="text-xs font-medium uppercase tracking-wide text-gray-500">
                  Description
                </div>
                <p className="mt-1 whitespace-pre-wrap text-sm text-gray-700">{r.description}</p>
              </div>
            )}
          </Card>

          <Card>
            <h3 className="mb-3 text-sm font-semibold text-gray-700">Services</h3>
            {(r.services ?? []).length === 0 ? (
              <p className="text-sm text-gray-400">No services.</p>
            ) : (
              <ul className="space-y-2">
                {r.services.map((s) => (
                  <li key={s.name} className="text-sm">
                    <span className="font-medium text-navy-900">{s.name}</span>
                    {s.description && <span className="text-gray-500"> — {s.description}</span>}
                    {s.details != null && (
                      <span className="ml-2 text-xs text-gray-400">
                        (details: {JSON.stringify(s.details)})
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Card>

          <Card>
            <div className="mb-3 flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-700">Downtimes</h3>
              {canManage && (
                <Link
                  href={`/downtimes/new?resource=${encodeURIComponent(r.name)}`}
                  className="text-xs font-medium text-brand-700 hover:underline"
                >
                  + Register downtime
                </Link>
              )}
            </div>
            <DowntimesTable downtimes={r.downtimes ?? []} canManage={canManage} />
          </Card>

          <Card>
            <div className="mb-3 flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-700">Contacts</h3>
              {canManage && <InviteContactButton entityKind="resource" entityName={r.name} />}
            </div>
            {(r.contacts ?? []).length === 0 ? (
              <p className="text-sm text-amber-600">
                No contacts registered — contacts are required for a complete registration.
              </p>
            ) : (
              <table className="min-w-full text-sm">
                <thead className="text-left text-xs uppercase tracking-wide text-gray-400">
                  <tr>
                    <th className="py-1 pr-4">Type</th>
                    <th className="py-1 pr-4">Name</th>
                    {canManage && <th className="py-1">Take over</th>}
                  </tr>
                </thead>
                <tbody>
                  {r.contacts.map((c, i) => (
                    <tr key={i} className="border-t border-gray-100">
                      <td className="py-1 pr-4 text-gray-700">
                        {c.contact_type}
                        {c.inherited_from && (
                          <span
                            className="ml-2 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500"
                            title={`Inherited from the ${c.inherited_from}`}
                          >
                            inherited · {c.inherited_from}
                          </span>
                        )}
                      </td>
                      <td className="py-1 pr-4 text-gray-700">{c.name || "—"}</td>
                      {canManage && (
                        <td className="py-1">
                          {/* Only own (non-inherited) slots can be taken over here;
                              inherited contacts are managed on the parent entity. */}
                          {!c.inherited_from && (
                            <ContactReplaceActions
                              entityKind="resource"
                              entityName={r.name}
                              contactType={c.contact_type}
                              rank={c.rank}
                            />
                          )}
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Card>
        </div>

        <div className="space-y-3">
          <Card>
            <h3 className="mb-2 text-sm font-semibold text-gray-700">Placement</h3>
            <div className="space-y-1 text-sm">
              <div>
                <span className="text-gray-400">Resource group: </span>
                <Link
                  href={`/resource-groups/detail?name=${encodeURIComponent(r.resource_group)}`}
                  className="text-brand-700 hover:underline"
                >
                  {r.resource_group}
                </Link>
              </div>
              <div>
                <span className="text-gray-400">Site: </span>
                <Link
                  href={`/sites/detail?name=${encodeURIComponent(r.site)}`}
                  className="text-brand-700 hover:underline"
                >
                  {r.site}
                </Link>
              </div>
              <div>
                <span className="text-gray-400">Facility: </span>
                <Link
                  href={`/facilities/detail?name=${encodeURIComponent(r.facility)}`}
                  className="text-brand-700 hover:underline"
                >
                  {r.facility}
                </Link>
              </div>
            </div>
          </Card>
          {r.vo_ownership != null && (
            <Card>
              <h3 className="mb-2 text-sm font-semibold text-gray-700">VO ownership</h3>
              <pre className="whitespace-pre-wrap text-xs text-gray-600">
                {JSON.stringify(r.vo_ownership, null, 2)}
              </pre>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}

export default function ResourceDetailPage() {
  return (
    <Suspense fallback={null}>
      <ResourceDetailView />
    </Suspense>
  );
}
